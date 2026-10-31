package mir_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
)

const isrCallbackSrc = `
import {Box} from std/alloc

fn withInert(b: &Box[Int]) {
    b.with((x: &Int) => {})
}

fn withMutInert(b: &mut Box[Int]) {
    b.withMut((x: &mut Int) => {})
}

fn withPanic(b: &Box[Int]) {
    b.with((x: &Int) => {
        let n: Int? = None
        _ = n.unwrap()
    })
}

fn withResult(b: &Box[Int]) -> Int {
    b.with((x: &Int) => 7)
}

fn withOpaque(b: &Box[Int], f: fn(&Int) -> Void) {
    b.with(f)
}

fn fixedSet(mut a: FixedArray[Int, 4]) {
    _ = a.set(1, 5)
}

fn fixedWithMut(mut a: FixedArray[Int, 4]) {
    _ = a.withMut(1, (x: &mut Int) => {})
}

fn fixedRead(a: FixedArray[Int, 4], i: usize) {
    _ = a[i]
}

export(interrupt) {
    pub fn tick() {}
}
`

// callFromTick makes tick call callee with a fresh local for every parameter, as a handler that had borrowed those values would.
func callFromTick(t *testing.T, prog *mir.Program, callee string) {
	t.Helper()
	tick, g := funcNamed(t, prog, "main.tick"), funcNamed(t, prog, "main."+callee)
	var args []midir.LocalID
	for _, p := range g.Params {
		tick.Locals = append(tick.Locals, midir.Local{Name: "held", Type: g.Locals[p].Type})
		args = append(args, midir.LocalID(len(tick.Locals)-1))
	}
	tick.Locals = append(tick.Locals, midir.Local{Name: "out", Type: g.Ret})
	blk := &tick.Blocks[0]
	blk.Insts = append(blk.Insts, midir.Inst{Op: midir.OpCall, Ent: g.Ent, Dst: midir.LocalID(len(tick.Locals) - 1), Args: args})
}

func TestInterruptLoweringCallbackPrimitives(t *testing.T) {
	cases := []struct{ callee, want string }{
		{"withInert", ""},
		{"withMutInert", ""},
		{"withPanic", "calls the runtime helper `panic`"},
		{"withResult", "returns its `Int` result in a fresh box"},
		{"fixedSet", ""},
		{"fixedWithMut", ""},
		{"fixedRead", "may panic on an out-of-bounds index"},
		{"withOpaque", "passes `with` a callback the lowering cannot see"},
	}
	for _, c := range cases {
		prog := planned(t, isrCallbackSrc)
		callFromTick(t, prog, c.callee)
		claims := mir.CheckInterruptLowering(prog)
		switch {
		case c.want == "" && len(claims) != 0:
			t.Errorf("%s: an inert callback through Box.with must pass: %s", c.callee, claims[0].Diag.Msg)
		case c.want != "" && (len(claims) != 1 || !strings.Contains(claims[0].Diag.Msg, c.want)):
			t.Errorf("%s: want %q: %+v", c.callee, c.want, claims)
		}
	}
}

func TestInterruptLoweringCallbackHasNoPositionalInertArgs(t *testing.T) {
	prog := planned(t, isrCallbackSrc)
	f := funcNamed(t, prog, "main.withInert")
	var call midir.Inst
	for _, in := range f.Blocks[0].Insts {
		if in.Op == midir.OpCall && prog.IsStdPrimitive(in.Ent) {
			call = in
		}
	}
	if call.Op != midir.OpCall || len(call.Args) != 2 {
		t.Fatalf("withInert must call a two-argument primitive: %+v", call)
	}
	callback := &midir.Func{Params: []midir.LocalID{0, 1}}
	if got := prog.IsrInertArgs(f, call, callback); got != nil {
		t.Errorf("a callback reached through a primitive must not take positional inert marks: %v", got)
	}
}
