package vm_test

import (
	"errors"
	"io"
	"sort"
	"strconv"
	"testing"
	"time"

	"kigumi/internal/backend/vm"
	"kigumi/internal/compiler/mir"
	mirbuild "kigumi/internal/compiler/mir/build"
	midir "kigumi/internal/compiler/mir/ir"
	mirplan "kigumi/internal/compiler/mir/plan"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

// sessionFakeOS gives the sessions their own `Host` lang item, so the tests
// exercise the VM session without depending on std (or on the current state
// of the std tree). The VM intercepts the `os.Host.exit` key before the
// fake body runs.
const sessionFakeOS = `pub type Host = {}

pub fn Host.exit(self, code: u8) -> Never {
    for {}
}
`

const sessionBaseSrc = `static mut counter: Int = 0

fn bump() -> Int {
    unsafe { counter = counter + 1 }
    unsafe { counter }
}

fn alive() -> Int {
    42
}

fn inc(x: Int) -> Int {
    x + 1
}

fn notB(b: Bool) -> Bool {
    !b
}

print "session"
`

const sessionExtraSrc = sessionBaseSrc + `
fn extra() -> Int {
    7
}
`

const sessionOtherSrc = `static mut renamed: Int = 0

fn bump() -> Int {
    unsafe { renamed = renamed + 1 }
    unsafe { renamed }
}

print "other"
`

const sessionCellsSrc = `type Payload = {
    pub a Int
    pub b String
}

type Pair = {
    pub a Int
    pub b String
}

fn boom() -> Int {
    let p = Payload { a: 1, b: "x" }
    let q = Payload { a: 2, b: "y" }
    p.a / (q.a - 2)
}

fn spin() -> Int {
    let mut i = 0
    for {
        i = i + 1
    }
}

fn quit() -> Void {
    host.exit(7)
}

fn pair() -> Pair {
    Pair { a: 1, b: "x" }
}

fn okCell() -> Int! {
    5
}

fn errCell() -> Int! {
    fail "nope"
}

fn alive() -> Int {
    42
}

print "cells"
`

// sessionCheck builds a module with the prelude-free universe and the tiny
// fake std/os, so a run stays fast and independent of the std tree.
func sessionCheck(t *testing.T, src string) *sem.Result {
	t.Helper()
	osTree := syntax.Parse(token.NewFile("std/os/session_fake.kg", []byte(sessionFakeOS)))
	if osTree.HasErrors() {
		t.Fatalf("fake std/os parse errors:\n%s", osTree.Dump())
	}
	tree := syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	if tree.HasErrors() {
		t.Fatalf("parse errors:\n%s", tree.Dump())
	}
	pkgs := map[string]*sem.Package{
		"std/os": {Path: "std/os", Module: "std", Std: true, Files: []*syntax.Tree{osTree}},
		"main":   {Path: "main", Files: []*syntax.Tree{tree}, Entry: tree},
	}
	paths := make([]string, 0, len(pkgs))
	for p := range pkgs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	mod := &sem.Module{}
	for _, p := range paths {
		mod.Packages = append(mod.Packages, pkgs[p])
	}
	res := sem.CheckWith(mod, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	return res
}

func buildSessionProgram(t *testing.T, src string) *mir.Program {
	t.Helper()
	res := sessionCheck(t, src)
	prog := mirbuild.All(res)
	if err := mirplan.Apply(prog, mirplan.Transform{Name: "plan-rc", Run: mirplan.RC}); err != nil {
		t.Fatalf("plan-rc: %v", err)
	}
	return prog
}

func sessionFunc(t *testing.T, prog *mir.Program, name string) *midir.Func {
	t.Helper()
	res := prog.R
	ent := res.PackageMember(res.PackageByPath("main"), name)
	if ent == 0 {
		t.Fatalf("no main.%s entity", name)
	}
	if f := prog.ByEnt[ent]; f != nil {
		return f
	}
	for _, f := range prog.Funcs {
		if f.Ent == ent {
			return f
		}
	}
	t.Fatalf("main.%s is not lowered", name)
	return nil
}

func newSession(t *testing.T, prog *mir.Program) *vm.Session {
	t.Helper()
	s, err := vm.NewSession(prog, vm.Options{}, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	return s
}

func runCell(t *testing.T, s *vm.Session, f *midir.Func) vm.Value {
	t.Helper()
	v, err := s.RunCell(f, nil)
	if err != nil {
		t.Fatalf("RunCell(%s): %v", f.Name, err)
	}
	return v
}

// Two cells on one Machine share the program's static and host state.
func TestSessionKeepsMachineStateAcrossCells(t *testing.T) {
	prog := buildSessionProgram(t, sessionBaseSrc)
	s := newSession(t, prog)
	defer s.Close()
	bump := sessionFunc(t, prog, "bump")
	for want := 1; want <= 2; want++ {
		v := runCell(t, s, bump)
		if got := v.Display(); got != strconv.Itoa(want) {
			t.Errorf("cell %d: got %q, want %d", want, got, want)
		}
		v.Release()
	}
}

// RunCell takes ownership of its argument Values and returns an owned result.
func TestSessionRunCellArgs(t *testing.T) {
	prog := buildSessionProgram(t, sessionBaseSrc)
	s := newSession(t, prog)
	defer s.Close()
	bump := sessionFunc(t, prog, "bump")
	inc := sessionFunc(t, prog, "inc")

	vm.TrackLive(true)
	defer vm.TrackLive(false)
	arg := runCell(t, s, bump)
	out, err := s.RunCell(inc, []vm.Value{arg})
	if err != nil {
		t.Fatalf("RunCell(inc): %v", err)
	}
	if got := out.Display(); got != "2" {
		t.Errorf("inc(1) = %q, want 2", got)
	}
	out.Release()
	if got := vm.LiveObjects(); got != 0 {
		t.Fatalf("argument transfer leaked %d objects", got)
	}
	if _, err := s.RunCell(inc, nil); err == nil {
		t.Error("RunCell with wrong arity succeeded")
	}
}

// Host-built Int/Bool values own a reference, survive RunCell args, and
// release safely; lastStatus is the motivating Int binding.
func TestSessionIntAndBoolConstructors(t *testing.T) {
	prog := buildSessionProgram(t, sessionBaseSrc)
	s := newSession(t, prog)
	defer s.Close()
	inc := sessionFunc(t, prog, "inc")
	notB := sessionFunc(t, prog, "notB")

	vm.TrackLive(true)
	defer vm.TrackLive(false)
	// A value outside the intern table is a real allocation, so a missed
	// release shows up as a live object.
	big := s.Int(1 << 40)
	if got := big.Display(); got != "1099511627776" {
		t.Fatalf("Int display = %q", got)
	}
	big.Release()
	big.Release() // safe twice
	if got := vm.LiveObjects(); got != 0 {
		t.Fatalf("releasing a host Int leaked %d objects", got)
	}

	arg := s.Int(1 << 40)
	out, err := s.RunCell(inc, []vm.Value{arg})
	if err != nil {
		t.Fatalf("RunCell(inc): %v", err)
	}
	if got := out.Display(); got != "1099511627777" {
		t.Errorf("inc(Int 2^40) = %q, want 1099511627777", got)
	}
	out.Release()

	b := s.Bool(true)
	if got := b.Display(); got != "true" {
		t.Errorf("Bool display = %q, want true", got)
	}
	nb, err := s.RunCell(notB, []vm.Value{b})
	if err != nil {
		t.Fatalf("RunCell(notB): %v", err)
	}
	if got := nb.Display(); got != "false" {
		t.Errorf("notB(true) = %q, want false", got)
	}
	nb.Release()
	if got := vm.LiveObjects(); got != 0 {
		t.Fatalf("constructor values left %d live objects", got)
	}
}

// A cell result's Ok/Err tag and error message are readable without
// assuming a field layout; released and zero Values are safe.
func TestSessionValueResult(t *testing.T) {
	prog := buildSessionProgram(t, sessionCellsSrc)
	s := newSession(t, prog)
	defer s.Close()
	okCell := sessionFunc(t, prog, "okCell")
	errCell := sessionFunc(t, prog, "errCell")

	var zero vm.Value
	if _, isResult := zero.ResultOk(); isResult {
		t.Error("zero Value reported a Result")
	}
	if got := zero.ErrorMessage(); got != "" {
		t.Errorf("zero Value ErrorMessage = %q", got)
	}

	okv := runCell(t, s, okCell)
	if ok, isResult := okv.ResultOk(); !ok || !isResult {
		t.Errorf("okCell ResultOk = (%v, %v), want (true, true)", ok, isResult)
	}
	if got := okv.ErrorMessage(); got != "" {
		t.Errorf("okCell ErrorMessage = %q, want empty", got)
	}
	okv.Release()
	if _, isResult := okv.ResultOk(); isResult {
		t.Error("released Value reported a Result")
	}

	errv := runCell(t, s, errCell)
	if ok, isResult := errv.ResultOk(); ok || !isResult {
		t.Errorf("errCell ResultOk = (%v, %v), want (false, true)", ok, isResult)
	}
	if got := errv.ErrorMessage(); got != "nope" {
		t.Errorf("errCell ErrorMessage = %q, want nope", got)
	}
	errv.Release()

	alive := sessionFunc(t, prog, "alive")
	v := runCell(t, s, alive)
	if _, isResult := v.ResultOk(); isResult {
		t.Error("plain Int reported a Result")
	}
	v.Release()
}

// A cell that panics leaves no live objects behind, and the session continues.
func TestSessionPanicSweepsValues(t *testing.T) {
	prog := buildSessionProgram(t, sessionCellsSrc)
	s := newSession(t, prog)
	defer s.Close()
	boom := sessionFunc(t, prog, "boom")
	alive := sessionFunc(t, prog, "alive")

	vm.TrackLive(true)
	defer vm.TrackLive(false)
	if _, err := s.RunCell(boom, nil); err == nil {
		t.Fatal("boom did not panic")
	} else {
		var p *vm.Panic
		if !errors.As(err, &p) {
			t.Fatalf("boom error = %T %v, want *vm.Panic", err, err)
		}
	}
	if got := vm.LiveObjects(); got != 0 {
		t.Fatalf("panic leaked %d live objects", got)
	}
	if got := vm.LiveStrays(); got != 0 {
		t.Fatalf("panic produced %d stray frees", got)
	}
	v := runCell(t, s, alive)
	if got := v.Display(); got != "42" {
		t.Errorf("after panic: got %q, want 42", got)
	}
	v.Release()
}

// Cancel interrupts an infinite cell promptly and leaves the session usable.
func TestSessionCancelStopsCell(t *testing.T) {
	prog := buildSessionProgram(t, sessionCellsSrc)
	s := newSession(t, prog)
	defer s.Close()
	spin := sessionFunc(t, prog, "spin")
	alive := sessionFunc(t, prog, "alive")

	done := make(chan error, 1)
	go func() {
		_, err := s.RunCell(spin, nil)
		done <- err
	}()
	// RunCell clears a stale flag when the cell starts, so keep asking
	// until it has taken effect.
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				s.Cancel()
				time.Sleep(time.Millisecond)
			}
		}
	}()
	start := time.Now()
	select {
	case err := <-done:
		close(stop)
		elapsed := time.Since(start)
		var c *vm.Cancelled
		if !errors.As(err, &c) {
			t.Fatalf("cancelled cell returned %T %v, want *vm.Cancelled", err, err)
		}
		if elapsed > time.Second {
			t.Fatalf("Cancel took %v, want under 1s", elapsed)
		}
	case <-time.After(5 * time.Second):
		close(stop)
		t.Fatal("Cancel did not stop the cell")
	}
	v := runCell(t, s, alive)
	if got := v.Display(); got != "42" {
		t.Errorf("after cancel: got %q, want 42", got)
	}
	v.Release()
}

// host.exit surfaces as a typed SessionExit instead of ending the process.
func TestSessionExitIsTyped(t *testing.T) {
	prog := buildSessionProgram(t, sessionCellsSrc)
	s := newSession(t, prog)
	defer s.Close()
	quit := sessionFunc(t, prog, "quit")
	_, err := s.RunCell(quit, nil)
	var exit *vm.SessionExit
	if !errors.As(err, &exit) {
		t.Fatalf("exit cell returned %T %v, want *vm.SessionExit", err, err)
	}
	if exit.Code != 7 {
		t.Errorf("exit code = %d, want 7", exit.Code)
	}
	alive := sessionFunc(t, prog, "alive")
	v := runCell(t, s, alive)
	if got := v.Display(); got != "42" {
		t.Errorf("after exit: got %q, want 42", got)
	}
	v.Release()
}

// UseProgram accepts a superset program, keeps globals, clears per-program
// tables, and rejects a program that changes the session's entity prefix.
func TestSessionUseProgram(t *testing.T) {
	base := buildSessionProgram(t, sessionBaseSrc)
	s := newSession(t, base)
	defer s.Close()
	bump := sessionFunc(t, base, "bump")
	v := runCell(t, s, bump)
	if got := v.Display(); got != "1" {
		t.Fatalf("first bump = %q, want 1", got)
	}
	v.Release()

	next := buildSessionProgram(t, sessionExtraSrc)
	if err := s.UseProgram(next); err != nil {
		t.Fatalf("UseProgram(superset): %v", err)
	}
	if s.Program() != next {
		t.Fatal("Program did not return the swapped program")
	}
	v = runCell(t, s, sessionFunc(t, next, "bump"))
	if got := v.Display(); got != "2" {
		t.Errorf("bump after swap = %q, want 2 (globals must survive)", got)
	}
	v.Release()
	v = runCell(t, s, sessionFunc(t, next, "extra"))
	if got := v.Display(); got != "7" {
		t.Errorf("new cell = %q, want 7", got)
	}
	v.Release()

	if err := s.UseProgram(nil); err == nil {
		t.Error("UseProgram(nil) succeeded")
	}
	if err := s.UseProgram(&mir.Program{}); err == nil {
		t.Error("UseProgram(empty) succeeded")
	}
	if err := s.UseProgram(&mir.Program{R: next.R}); err == nil {
		t.Error("UseProgram(program without entry) succeeded")
	}
	if err := s.UseProgram(buildSessionProgram(t, sessionOtherSrc)); err == nil {
		t.Error("UseProgram(incompatible) succeeded")
	}
	// The rejected swaps left the session on the superset program.
	v = runCell(t, s, sessionFunc(t, next, "alive"))
	if got := v.Display(); got != "42" {
		t.Errorf("after rejected swap: got %q, want 42", got)
	}
	v.Release()
}

// A wrapper record's fields can be detached without double-releasing them.
func TestSessionValueFieldsAndTakeFields(t *testing.T) {
	prog := buildSessionProgram(t, sessionCellsSrc)
	s := newSession(t, prog)
	defer s.Close()
	pair := sessionFunc(t, prog, "pair")
	vm.TrackLive(true)
	defer vm.TrackLive(false)

	v := runCell(t, s, pair)
	if got := v.RecordLen(); got != 2 {
		t.Fatalf("RecordLen = %d, want 2", got)
	}
	f0, ok := v.Field(0)
	if !ok || f0.Display() != "1" {
		t.Fatalf("Field(0) = %v %v, want 1", f0.Display(), ok)
	}
	if _, ok := v.Field(2); ok {
		t.Error("Field(2) succeeded out of range")
	}
	fields := v.TakeFields()
	if !v.IsNil() {
		t.Error("TakeFields left the container owner set")
	}
	if len(fields) != 2 {
		t.Fatalf("TakeFields returned %d values, want 2", len(fields))
	}
	if got := fields[0].Display(); got != "1" {
		t.Errorf("field 0 = %q, want 1", got)
	}
	if got := fields[1].Display(); got != "x" {
		t.Errorf("field 1 = %q, want x", got)
	}
	f0.Release()
	for i := range fields {
		fields[i].Release()
	}
	if got := vm.LiveObjects(); got != 0 {
		t.Fatalf("detaching fields leaked %d objects", got)
	}
}

// Close is safe to call twice and right after a panicking cell.
func TestSessionCloseAfterPanic(t *testing.T) {
	prog := buildSessionProgram(t, sessionCellsSrc)
	s := newSession(t, prog)
	boom := sessionFunc(t, prog, "boom")
	if _, err := s.RunCell(boom, nil); err == nil {
		t.Fatal("boom did not panic")
	}
	s.Close()
	s.Close()
	if _, err := s.RunCell(boom, nil); !errors.Is(err, vm.ErrSessionClosed) {
		t.Errorf("RunCell after Close = %v, want ErrSessionClosed", err)
	}
}
