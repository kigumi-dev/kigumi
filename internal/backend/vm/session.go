package vm

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// Session errors. A caller that sees one of these can keep using the
// session; only SessionExit ends it.
var (
	ErrSessionBusy   = errors.New("vm: session is busy")
	ErrSessionClosed = errors.New("vm: session is closed")
	ErrInvalidProg   = errors.New("vm: invalid session program")
)

// SessionExit reports that a cell asked the host to exit. The caller should
// terminate the REPL with Code; the session is still usable otherwise.
type SessionExit struct{ Code int }

func (e *SessionExit) Error() string { return fmt.Sprintf("exit status %d", e.Code) }

// Cancelled is the error RunCell returns after Cancel interrupted a cell.
type Cancelled struct{}

func (*Cancelled) Error() string { return "vm: cell cancelled" }

// cancelEvery is how many instructions pass between two reads of the shared
// cancellation flag: one atomic load per 256 instructions on the hot path.
const cancelEvery = 256

// Session owns one persistent Machine across cells. Values handed out by the
// session stay valid only while the Session lives; the driver keeps them in
// its own symbol table.
type Session struct {
	m  *Machine
	mu sync.Mutex
	// running is set for the duration of RunCell, so a second call (or a
	// UseProgram) is refused instead of corrupting the interpreter state.
	running atomic.Bool
	closed  atomic.Bool
	// entityMap carries a held value's old entity ids to the new program
	// after the checker renumbered a declaration.
	entityMap map[sem.EntityID]sem.EntityID
}

// NewSession starts a session on prog. The Machine is created once: later
// cells reuse its globals, host registrations and lazily built tables.
func NewSession(prog *mir.Program, opts Options, out, errOut io.Writer) (*Session, error) {
	if err := validateProgram(prog); err != nil {
		return nil, err
	}
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = io.Discard
	}
	return &Session{m: newMachine(prog, opts, out, errOut)}, nil
}

// Program is the program the session currently runs.
func (s *Session) Program() *mir.Program {
	if s == nil || s.m == nil {
		return nil
	}
	return s.m.p
}

// UseProgram swaps in a program built from the session's source plus a new
// cell. It rejects a program that changes the ids or layouts of the session
// package's entities (standard-library ids may move) and clears the lazy
// tables keyed by entity or function id; globals and host registrations stay.
func (s *Session) UseProgram(prog *mir.Program) error {
	if s == nil || s.m == nil {
		return ErrSessionClosed
	}
	if err := validateProgram(prog); err != nil {
		return err
	}
	if s.running.Load() {
		return ErrSessionBusy
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return ErrSessionClosed
	}
	remap, err := compatibleResults(s.m.r, prog.R)
	if err != nil {
		return err
	}
	if err := s.compatibleGlobals(prog.R); err != nil {
		return err
	}
	s.entityMap = remap
	if len(s.m.frames) != 0 || s.m.depth != 0 {
		return ErrSessionBusy
	}
	s.m.p = prog
	s.m.r = prog.R
	// newMachine does this once for the initial program; swapping must
	// mirror it, since the VM reads the flag during unwind planning.
	if !s.m.opts.Sandbox {
		prog.ContainsDropPanics()
	}
	s.clearCaches()
	s.resetTransient()
	return nil
}

// RunCell calls fn with args on the session's Machine. Ownership of args
// passes to the cell; the returned Value owns its reference. A cell panic is
// recovered, every frame it left behind is swept, and the panic is returned
// without leaking its locals; the session stays usable.
func (s *Session) RunCell(fn *midir.Func, args []Value) (Value, error) {
	if s == nil || s.m == nil {
		return Value{}, ErrSessionClosed
	}
	if fn == nil {
		return Value{}, fmt.Errorf("%w: nil cell function", ErrInvalidProg)
	}
	if len(args) != len(fn.Params) {
		return Value{}, fmt.Errorf("%w: cell takes %d arguments, got %d", ErrInvalidProg, len(fn.Params), len(args))
	}
	if !s.running.CompareAndSwap(false, true) {
		return Value{}, ErrSessionBusy
	}
	defer s.running.Store(false)
	if s.closed.Load() {
		return Value{}, ErrSessionClosed
	}
	s.mu.Lock()
	s.resetTransient()
	s.m.sh.cancel.Store(false)
	s.mu.Unlock()

	objs := make([]*obj, len(args))
	for i := range args {
		objs[i] = args[i].v
		args[i] = Value{}
	}

	var (
		out *obj
		err error
	)
	func() {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			// A drop hook must not race the cancellation that unwound
			// the cell, so the flag is cleared before the sweep.
			s.m.sh.cancel.Store(false)
			switch e := r.(type) {
			case *Panic:
				err = s.sweepAfter(e, 0)
			case *Unsupported:
				err = s.sweepAfter(e, 0)
			case *exitSignal:
				err = s.sweepAfter(&SessionExit{Code: e.code}, 0)
			case *Cancelled:
				err = s.sweepAfter(e, 0)
			default:
				s.sweepAfter(&Panic{Msg: fmt.Sprint(r)}, 0)
				panic(r)
			}
		}()
		out = s.m.call(fn, objs)
	}()
	if err != nil {
		s.resetTransient()
		return Value{}, err
	}
	v := Value{s: s, v: out}
	s.resetTransient()
	return v, nil
}

// sweepAfter releases the frames the failed cell left on the stack, then
// returns primary unless a drop hook itself panicked, in which case the
// joined panic is the more useful error.
func (s *Session) sweepAfter(primary error, base int) error {
	m := s.m
	var marker *Panic
	if p, ok := primary.(*Panic); ok {
		marker = p
	} else {
		marker = &Panic{Msg: primary.Error()}
	}
	outer := m.sweepFault
	m.sweepFault = marker
	m.sweepFrames(base)
	got := m.sweepFault
	m.sweepFault = outer
	if got != nil && got != marker {
		return got
	}
	return primary
}

// Cancel asks the executing cell to stop at the next cancellation check.
// It is a no-op when the session is idle: the flag is cleared when a cell
// starts, so a stale cancellation never kills the next cell.
func (s *Session) Cancel() {
	if s == nil || s.m == nil || s.closed.Load() {
		return
	}
	s.m.sh.cancel.Store(true)
}

// Reset discards any leftover per-cell interpreter state. It is a no-op
// while a cell runs.
func (s *Session) Reset() {
	if s == nil || s.m == nil || s.running.Load() {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return
	}
	s.resetTransient()
}

// Quiesce cancels and drains the task pools a cell may have left behind,
// without ending the session.
func (s *Session) Quiesce() {
	if s == nil || s.m == nil {
		return
	}
	s.m.sh.cancel.Store(true)
	s.m.drainPools()
	s.m.joinOrphans()
	s.m.sh.cancel.Store(false)
	s.m.flushReleases()
}

// Close ends the session: outstanding task pools are stopped and drained and
// any interpreter state left by a panic is released. It is idempotent and
// safe to call after a panic.
func (s *Session) Close() {
	if s == nil || s.m == nil || !s.closed.CompareAndSwap(false, true) {
		return
	}
	defer func() {
		// Close has no error return, so a drop hook that panics during
		// the final sweep must not escape it.
		_ = recover()
	}()
	s.m.sh.cancel.Store(true)
	s.m.flushReleases()
	s.m.drainPools()
	s.m.joinOrphans()
	s.m.sweepFrames(0)
	s.m.sh.cancel.Store(false)
	s.resetTransient()
}

// checkCancel is the instruction loop's cancellation check: the shared flag
// and the timeout drain budget are read once every cancelEvery instructions.
func (m *Machine) checkCancel() {
	m.cancelTicks++
	if m.cancelTicks&(cancelEvery-1) != 0 {
		return
	}
	if m.sh.cancel.Load() {
		panic(&Cancelled{})
	}
	if m.sh.drainLeft.Load() != 0 && m.sh.drainLeft.Add(-cancelEvery) <= 0 {
		panic(&Cancelled{})
	}
}

// resetTransient returns the Machine to the state a fresh call expects,
// keeping pools, globals, host registrations and lazily built program tables.
func (s *Session) resetTransient() {
	m := s.m
	m.stack = m.stack[:0]
	m.frames = m.frames[:0]
	m.depth = 0
	m.stepsLeft = 0
	m.cur = nil
	m.curLane = 0
	m.pending = m.pending[:0]
	m.site, m.siteFn, m.curFn, m.curInst = nil, nil, nil, nil
	m.sweepFault = nil
	m.inflight = nil
	m.outerDepth, m.joinDepth, m.closeDepth = 0, 0, 0
	m.cancelTicks = 0
	m.sh.cancel.Store(false)
}

// clearCaches drops every table whose keys or values belong to the old
// program. Globals and host registrations survive UseProgram.
func (s *Session) clearCaches() {
	m := s.m
	m.sh.derived.Lock()
	m.sh.named = map[string]sem.EntityID{}
	m.sh.drops = map[sem.EntityID]*midir.Func{}
	m.sh.borrowMask = map[sem.EntityID]uint64{}
	m.sh.underlying = map[sem.TypeID]sem.TypeID{}
	m.sh.derived.Unlock()
	m.sh.codes = sync.Map{}
	m.witnessed = nil
	m.valueSites = nil
	m.boxFacts = nil
	m.boxParams = nil
}

// Value is one owned reference to a VM value. The zero Value is nil and
// every method tolerates it. A Value must not outlive its Session.
type Value struct {
	s *Session
	v *obj
}

// Int returns a new owned Value holding an Int. The REPL uses it for
// synthetic session bindings such as lastStatus.
func (s *Session) Int(i int64) Value {
	if s == nil || s.m == nil {
		return Value{}
	}
	return Value{s: s, v: mkInt(i, 0)}
}

// Bool returns a new owned Value holding a Bool.
func (s *Session) Bool(b bool) Value {
	if s == nil || s.m == nil {
		return Value{}
	}
	return Value{s: s, v: mkBool(b)}
}

// IntValue returns an Int value's payload. ok is false for a non-Int.
func (v Value) IntValue() (int64, bool) {
	if v.s == nil || v.v == nil {
		return 0, false
	}
	d := deref(v.v)
	if d == nil || d.k != kInt {
		return 0, false
	}
	return d.i, true
}

// ResultOk reports the tag of a Result-shaped variant: ok says whether it is
// the Ok variant and isResult whether the value is a Result at all. Both are
// false for a nil/released Value and for every non-Result value.
func (v Value) ResultOk() (ok bool, isResult bool) {
	if v.s == nil || v.v == nil {
		return false, false
	}
	d := deref(v.v)
	if d == nil || d.k != kVariant {
		return false, false
	}
	resultEnt := v.s.m.r.Types.ResultEnt()
	if resultEnt == 0 {
		return false, false
	}
	switch d.ent {
	case v.s.m.variantOf(resultEnt, "Ok"):
		return true, true
	case v.s.m.variantOf(resultEnt, "Err"):
		return false, true
	}
	return false, false
}

// ErrorMessage renders the failure of an Err Result, or of an error value
// itself, through its `message` method. It falls back to Display, and returns
// "" for an Ok Result and for a nil/released Value.
func (v Value) ErrorMessage() string {
	if v.s == nil || v.v == nil {
		return ""
	}
	s := v.s
	e := v.v
	if ok, isResult := v.ResultOk(); isResult {
		if ok {
			return ""
		}
		if d := recordOf(e); d != nil && len(d.fields) > 0 {
			e = d.fields[0]
		}
	}
	if s.running.Load() {
		return s.m.display(e)
	}
	var msg string
	func() {
		defer func() { _ = recover() }()
		m := s.m.errorMessage(e)
		if m == nil {
			return
		}
		msg = s.m.display(m)
		s.m.release(m)
	}()
	if msg == "" {
		return s.m.display(e)
	}
	return msg
}

// IsNil reports whether the Value holds no reference.
func (v Value) IsNil() bool { return v.v == nil }

// Retain returns another owned reference to the same value.
func (v Value) Retain() Value {
	if v.s == nil || v.v == nil {
		return Value{}
	}
	return Value{s: v.s, v: retain(v.v)}
}

// Release drops this Value's reference. It is safe to call twice.
func (v *Value) Release() {
	if v == nil || v.s == nil || v.v == nil {
		return
	}
	s, x := v.s, v.v
	v.s, v.v = nil, nil
	s.m.release(x)
}

// Display renders the value the way `debug`/print interpolation does.
func (v Value) Display() string {
	if v.s == nil || v.v == nil {
		return "<nil>"
	}
	return v.s.m.display(v.v)
}

// recordOf unwraps the containers a record may travel in; nil for any
// other value.
func recordOf(v *obj) *obj {
	d := deref(v)
	if d != nil && d.k == kBox {
		d = d.inner
	}
	if d == nil || (d.k != kRecord && d.k != kVariant) {
		return nil
	}
	return d
}

// RecordLen is the number of fields of a record or variant payload, or 0
// for any other value.
func (v Value) RecordLen() int {
	if v.s == nil || v.v == nil {
		return 0
	}
	if d := recordOf(v.v); d != nil {
		return len(d.fields)
	}
	return 0
}

// Field returns field i as a separately owned Value. The receiver keeps its
// own reference.
func (v Value) Field(i int) (Value, bool) {
	if v.s == nil || v.v == nil {
		return Value{}, false
	}
	d := recordOf(v.v)
	if d == nil || i < 0 || i >= len(d.fields) {
		return Value{}, false
	}
	return Value{s: v.s, v: v.s.m.field(v.v, i)}, true
}

// TakeFields detaches every field of a record or variant into its own Value
// and releases the container, so a driver can move a wrapper's fields into
// its symbol table without double-releasing them. The receiver becomes nil.
// For a non-record value it returns nil and releases the value.
func (v *Value) TakeFields() []Value {
	if v == nil || v.s == nil || v.v == nil {
		return nil
	}
	s, x := v.s, v.v
	v.s, v.v = nil, nil
	d := recordOf(x)
	if d == nil {
		s.m.release(x)
		return nil
	}
	out := make([]Value, len(d.fields))
	for i := range d.fields {
		out[i] = Value{s: s, v: s.m.fieldMove(x, i)}
	}
	s.m.release(x)
	return out
}

// entityKey names a declaration by its package, kind, name and parent, not
// by the id the checker happened to allocate.
func entityKey(r *sem.Result, id sem.EntityID) string {
	if id == 0 || int(id) >= len(r.Entities) {
		return ""
	}
	e := &r.Entities[id]
	pkg := ""
	if int(e.Pkg) < len(r.Packages) {
		pkg = r.Packages[e.Pkg].Path
	}
	parent := ""
	if e.Parent != 0 {
		parent = entityKey(r, e.Parent)
	}
	return pkg + "\x00" + fmt.Sprint(e.Kind) + "\x00" + e.Name + "\x00" + parent
}

// Remap applies the last UseProgram's entity-key mapping to a held value,
// including its nested records and arrays. It is safe to call more than once.
func (s *Session) Remap(v Value) {
	if s == nil || v.s != s || v.v == nil || len(s.entityMap) == 0 {
		return
	}
	s.remapObj(v.v, map[*obj]bool{})
}

func (s *Session) remapObj(v *obj, seen map[*obj]bool) {
	if v == nil || seen[v] {
		return
	}
	seen[v] = true
	if v.ent != 0 {
		if n, ok := s.entityMap[v.ent]; ok {
			v.ent = n
		}
	}
	for _, f := range v.fields {
		s.remapObj(f, seen)
	}
	s.remapObj(v.inner, seen)
}

func validateProgram(prog *mir.Program) error {
	if prog == nil || prog.R == nil {
		return fmt.Errorf("%w: no result", ErrInvalidProg)
	}
	if prog.Entry == nil {
		return fmt.Errorf("%w: no entry function", ErrInvalidProg)
	}
	if len(prog.R.Entities) <= 1 {
		return fmt.Errorf("%w: empty entity table", ErrInvalidProg)
	}
	if len(prog.Skipped) != 0 {
		return fmt.Errorf("%w: %d bodies were not lowered", ErrInvalidProg, len(prog.Skipped))
	}
	return nil
}

// stableKind reports whether an entity's id and type layout must survive a
// re-check of the session source. Locals, params, closures, tests and the
// synthetic implicit main are recreated per compilation and are skipped.
func stableKind(k sem.EntityKind) bool {
	switch k {
	case sem.EntType, sem.EntAlias, sem.EntInterface, sem.EntConstraint,
		sem.EntVariant, sem.EntField, sem.EntFn, sem.EntConst, sem.EntStatic:
		return true
	}
	return false
}

// compatibleResults accepts new only when every session-package entity a
// stored value can mention still has the same id, name, kind and layout.
// Standard-library ids may shift when a cell adds a declaration (the
// checker assigns them lazily); the session's own packages must stay a
// strict prefix, which is what the driver guarantees by appending source.
func compatibleResults(old, new *sem.Result) (map[sem.EntityID]sem.EntityID, error) {
	if old == nil || new == nil {
		return nil, fmt.Errorf("%w: missing result", ErrInvalidProg)
	}
	newByKey := map[string]sem.EntityID{}
	for id := 1; id < len(new.Entities); id++ {
		e := &new.Entities[id]
		if e.Kind == sem.EntNone || !stableKind(e.Kind) || strings.HasPrefix(e.Name, "__repl_") {
			continue
		}
		if k := entityKey(new, sem.EntityID(id)); k != "" {
			newByKey[k] = sem.EntityID(id)
		}
	}
	remap := map[sem.EntityID]sem.EntityID{}
	for id := 1; id < len(old.Entities); id++ {
		o := &old.Entities[id]
		if o.Kind == sem.EntNone || !stableKind(o.Kind) || strings.HasPrefix(o.Name, "__repl_") {
			continue
		}
		if o.Parent != 0 && strings.HasPrefix(old.Entity(o.Parent).Name, "__repl_") {
			continue
		}
		key := entityKey(old, sem.EntityID(id))
		nid, ok := newByKey[key]
		if !ok {
			if old.Packages[o.Pkg].Std {
				continue
			}
			return nil, fmt.Errorf("%w: entity %s %q in package %q is missing from the new program", ErrInvalidProg, o.Kind, o.Name, old.Packages[o.Pkg].Path)
		}
		n := &new.Entities[nid]
		if o.Kind != n.Kind || o.Name != n.Name {
			if old.Packages[o.Pkg].Std {
				continue
			}
			return nil, fmt.Errorf("%w: entity %s %q changed at its key", ErrInvalidProg, o.Kind, o.Name)
		}
		if old.TypeString(o.Type) != new.TypeString(n.Type) {
			if old.Packages[o.Pkg].Std {
				continue
			}
			return nil, fmt.Errorf("%w: entity %s %q changed type from %s to %s", ErrInvalidProg, o.Kind, o.Name, old.TypeString(o.Type), new.TypeString(n.Type))
		}
		if o.Kind == sem.EntType && !old.Packages[o.Pkg].Std {
			ot, nt := old.TypeDecl(sem.EntityID(id)), new.TypeDecl(nid)
			if ot.Form != nt.Form || ot.Layout != nt.Layout || ot.Align != nt.Align ||
				len(ot.Fields) != len(nt.Fields) || len(ot.Variants) != len(nt.Variants) {
				return nil, fmt.Errorf("%w: type %q changed layout", ErrInvalidProg, o.Name)
			}
		}
		remap[sem.EntityID(id)] = nid
	}
	return remap, nil
}

// compatibleGlobals checks the statics whose ids key the surviving global
// table: the value at an id belongs to the session and cannot be re-keyed.
func (s *Session) compatibleGlobals(next *sem.Result) error {
	m := s.m
	m.sh.store.Lock()
	ids := make([]sem.EntityID, 0, len(m.sh.globals))
	for id := range m.sh.globals {
		ids = append(ids, id)
	}
	m.sh.store.Unlock()
	for _, id := range ids {
		if int(id) >= len(next.Entities) {
			return fmt.Errorf("%w: static entity %d is missing from the new program", ErrInvalidProg, id)
		}
		o, n := &m.r.Entities[id], &next.Entities[id]
		if o.Kind != n.Kind || o.Name != n.Name || m.r.Packages[o.Pkg].Path != next.Packages[n.Pkg].Path {
			return fmt.Errorf("%w: static entity %d changed from %s %q to %s %q", ErrInvalidProg, id, o.Kind, o.Name, n.Kind, n.Name)
		}
	}
	return nil
}
