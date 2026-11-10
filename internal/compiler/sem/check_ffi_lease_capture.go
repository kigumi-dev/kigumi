package sem

import (
	"strings"

	"kigumi/internal/compiler/syntax"
)

type leaseWalk struct {
	c        *checker
	fn       string
	captured map[EntityID]bool
	tainted  map[EntityID]bool
}

func (c *checker) ffiLeaseCapture(fn EntityID, args []syntax.NodeID) {
	e := &c.r.Entities[fn]
	if c.r.Packages[e.Pkg].Path != "std/ffi" || (e.Name != "withAddress" && e.Name != "withAddressConst") || len(args) != 2 {
		return
	}
	lam := c.t.View(args[1]).ArgValue().ID()
	for c.t.Kind(lam) == syntax.Paren {
		lam = c.t.View(lam).AsParen().Inner().ID()
	}
	ent := c.info.Defs[lam]
	if c.t.Kind(lam) != syntax.Lambda || ent == 0 {
		return
	}
	info := c.r.closure(ent)
	if len(info.Params) != 1 {
		return
	}
	w := &leaseWalk{c: c, fn: e.Name, captured: map[EntityID]bool{}, tainted: map[EntityID]bool{info.Params[0]: true}}
	for _, cap := range info.Captures {
		w.captured[cap.Local] = true
	}
	w.walk(c.t.View(lam).AsLambda().Body().ID())
}

func (w *leaseWalk) walk(n syntax.NodeID) {
	t := w.c.t
	switch t.Kind(n) {
	case syntax.CallExpr, syntax.WsCallExpr:
		w.call(n)
	case syntax.AssignStmt:
		a := t.View(n).AsAssign()
		w.store(n, a.Target().ID(), a.Value().ID())
	case syntax.MatchExpr:
		if w.mentions(t.View(n).AsMatch().Scrutinee().ID()) {
			for arm := range t.View(n).AsMatch().Arms().IDs() {
				w.taintDefs(matchArm(t, arm).Pattern)
			}
		}
	case syntax.IfLetExpr:
		if s := ifLet(t, n); w.mentions(s.Init) {
			w.taintDefs(s.Pattern)
		}
	case syntax.LetStmt:
		if init := t.View(n).AsLet().Init().ID(); init != 0 && w.mentions(init) {
			w.taintDefs(t.View(n).AsLet().Pattern().ID())
		}
	}
	for child := range t.View(n).Children() {
		w.walk(child.ID())
	}
}

func (w *leaseWalk) call(n syntax.NodeID) {
	t := w.c.t
	var args []syntax.NodeID
	var callee syntax.NodeID
	if t.Kind(n) == syntax.CallExpr {
		v := t.View(n).AsCall()
		callee, args = v.Callee().ID(), v.Args().AppendIDs(nil)
	} else {
		v := t.View(n).AsWsCall()
		callee, args = v.Callee().ID(), v.Args().AppendIDs(nil)
	}
	carries := false
	for _, a := range args {
		carries = carries || w.mentions(a)
	}
	if !carries {
		return
	}
	if t.Kind(callee) == syntax.MemberExpr {
		base := t.View(callee).AsMember().Base().ID()
		if name, ok := w.capturedRoot(base); ok {
			w.c.errAt(n, cLeaseStored, w.fn, name)
			return
		}
		if ent := w.rootEnt(base); ent != 0 {
			w.tainted[ent] = true
		}
	}
	for _, a := range args {
		v := t.View(a).ArgValue().ID()
		if t.Kind(v) == syntax.BorrowExpr && t.View(v).AsBorrow().Flags()&syntax.FlagMut != 0 {
			if name, ok := w.capturedRoot(t.View(v).AsBorrow().Operand().ID()); ok {
				w.c.errAt(n, cLeaseStored, w.fn, name)
				return
			}
		}
	}
	// A callee the check cannot see through can store the pointer in state
	// that outlives the call, which the direct assign/return/borrow forms
	// above never see: `stash(h, p)` writes p into the record h points at,
	// and `h` reaches the call as an ordinary `&mut` binding, not a borrow
	// expression. Rejecting the hand-off itself closes the whole family
	// (callee parameter, closure, field, Result) instead of enumerating
	// syntax shapes. std is trusted (it is checked with these same rules)
	// and a C declaration keeps C's own contract.
	if name, ok := w.retainingCallee(callee); ok {
		w.c.errAt(n, cLeasePassed, w.fn, name)
	}
}

// retainingCallee names a call target that may retain a lease pointer: a
// Kigumi body outside std, or a function value whose body is out of reach
// here. Construction, intrinsics, std and extern(C) declarations pass.
func (w *leaseWalk) retainingCallee(callee syntax.NodeID) (string, bool) {
	ent := w.c.info.Uses[callee]
	if ent == 0 {
		return "", false
	}
	e := &w.c.r.Entities[ent]
	switch e.Kind {
	case EntFn:
		path := w.c.r.Packages[e.Pkg].Path
		if path == "std" || strings.HasPrefix(path, "std/") {
			return "", false
		}
		if w.c.r.Fn(ent).Body == 0 {
			return "", false
		}
		return e.Name, true
	case EntClosure, EntLocal, EntParam, EntField, EntFieldParam:
		return e.Name, true
	}
	return "", false
}

func (w *leaseWalk) store(n, target, value syntax.NodeID) {
	if !w.mentions(value) {
		return
	}
	if name, ok := w.capturedRoot(target); ok {
		w.c.errAt(n, cLeaseStored, w.fn, name)
		return
	}
	if ent := w.rootEnt(target); ent != 0 {
		w.tainted[ent] = true
	}
}

func (w *leaseWalk) taintDefs(n syntax.NodeID) {
	if ent := w.c.info.Defs[n]; ent != 0 {
		w.tainted[ent] = true
	}
	for child := range w.c.t.View(n).Children() {
		w.taintDefs(child.ID())
	}
}

func (w *leaseWalk) mentions(n syntax.NodeID) bool {
	if w.tainted[w.c.info.Uses[n]] {
		return true
	}
	found := false
	for child := range w.c.t.View(n).Children() {
		if w.mentions(child.ID()) {
			found = true
			break
		}
	}
	return found && w.canCarry(n)
}

func (w *leaseWalk) canCarry(n syntax.NodeID) bool {
	ty := w.c.vars.resolve(w.c.info.Types[n])
	if ty == 0 {
		return true
	}
	return w.c.r.mayContainPtr(ty, map[TypeID]bool{}) || w.c.r.mayContainFn(ty, map[TypeID]bool{})
}

func (w *leaseWalk) rootEnt(n syntax.NodeID) EntityID {
	for n != 0 {
		if ent := w.c.info.Uses[n]; ent != 0 && w.c.t.Kind(n) != syntax.MemberExpr {
			return ent
		}
		next := syntax.NodeID(0)
		for child := range w.c.t.View(n).Children() {
			next = child.ID()
			break
		}
		n = next
	}
	return 0
}

func (w *leaseWalk) capturedRoot(n syntax.NodeID) (string, bool) {
	ent := w.rootEnt(n)
	if ent == 0 {
		return "", false
	}
	e := &w.c.r.Entities[ent]
	return e.Name, w.captured[ent] || e.Kind == EntStatic
}
