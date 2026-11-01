package ownership

import (
	"sort"

	midir "kigumi/internal/compiler/mir/ir"
)

func (a *owner) movedFields(root midir.LocalID, st *dstate) []int {
	var out []int
	for idx, rep := range a.pl.children[root] {
		if gone(st.field.get(rep)) || a.tracked[rep] && gone(st.val.get(rep)) || len(a.movedFields(rep, st)) > 0 {
			out = append(out, idx)
		}
	}
	sort.Ints(out)
	return out
}

// checkBorrowedWhole requires a `mut self` receiver (or a closure's lent
// capture of one) to be whole again by every return: unlike an owned
// place, it is never dropped here, so a field left moved out would vanish
// from the caller's value with no drop and no diagnostic to show for it.
func gone(s ownState) bool { return s == ownMoved || s == ownConflict }

func (a *owner) checkBorrowedWhole(st *dstate, report bool) {
	if !report {
		return
	}
	for root := range a.pl.children {
		if !a.f.Locals[root].Borrowed {
			continue
		}
		fields := a.movedFields(root, st)
		if len(fields) == 0 {
			continue
		}
		rep := a.pl.children[root][fields[0]]
		a.checkWhole(a.defNode[rep], root, st, report)
	}
}

func (a *owner) useArgs(in *midir.Inst, st *dstate, report bool) {
	a.cellStep(in, st, report)
	a.assignHeld(in, st, report)
	if in.Op == midir.OpClosure {
		a.captureWhole(in, st, report)
	}
	if in.Op == midir.OpCopy && in.Guarded {
		st.rel.del(a.pl.canon(in.Args[0]))
	}
	a.checkArmTake(in, report)
	if a.movesIntoBinding(in) {
		l := a.pl.canon(in.Args[0])
		a.checkWhole(in.Node, l, st, report)
		a.checkDropOwner(in.Node, l, report)
		a.checkGuardHeld(in, l, st, report && !a.fieldMoved(in.Args[0]), 0)
		a.moveGuardSource(in, l, st, report)
		if in.Share {
			// A shared scrutinee temporary keeps its own drop, so the binding takes the field without emptying it.
			st.field.set(l, ownMoved)
			st.at.set(l, in.Node)
		} else {
			st.val.set(l, ownMoved)
			st.at.set(l, in.Node)
		}
		a.noteMove(in, l, report && !a.fieldMoved(in.Args[0]))
		return
	}
	for argi, arg := range in.Args {
		l := a.pl.canon(arg)
		if in.Op == midir.OpBorrow && in.Index == 1 || (in.Op == midir.OpSetField || in.Op == midir.OpSetIndex) && argi == 0 {
			a.checkGuardHeld(in, l, st, report, 0)
		}
		if !a.tracked[l] {
			// A `mut self` receiver is an untracked cell; a whole use still needs every field back.
			if a.f.Locals[l].Borrowed && wholeUse(in.Op) {
				a.checkWhole(in.Node, l, st, report)
			}
			continue
		}
		a.check(in, l, st.val.get(l), report)
		switch {
		case a.pl.isPlace(arg) && (ConsumesArg(a.p.R, a.p.ByEnt, in, argi) || PrimitiveTakes(a.p.R, in, argi) || Constructs(in.Op)):
			a.checkWhole(in.Node, l, st, report)
			a.checkDropOwner(in.Node, l, report)
			a.checkGuardHeld(in, l, st, report && !a.fieldMoved(arg), 0)
			a.moveGuardSource(in, l, st, report)
			st.val.set(l, ownMoved)
			st.at.set(l, in.Node)
			a.noteMove(in, l, report)
		case wholeUse(in.Op):
			a.checkWhole(in.Node, l, st, report)
		}
	}
}

func wholeUse(op midir.Opcode) bool {
	return op == midir.OpCall || op == midir.OpCallValue || op == midir.OpBuiltin || op == midir.OpBorrow || op == midir.OpTag || op == midir.OpIsVariant
}

func (a *owner) guardedMove(in *midir.Inst, st *dstate, report bool) {
	if !in.Guarded || len(in.Args) == 0 {
		return
	}
	arg := in.Args[0]
	if in.Str == GuardFailed {
		st.rel.set(a.pl.canon(arg), ownValid)
		return
	}
	a.cellConsume(in, arg, st, report)
	if l := a.pl.canon(arg); a.holdable(arg) && a.tracked[l] {
		a.checkWhole(in.Node, l, st, report)
		a.checkDropOwner(in.Node, l, report)
		var own midir.LocalID
		if len(in.Args) > 1 {
			own = in.Args[1]
		}
		a.checkGuardHeld(in, l, st, report, own)
		a.moveGuardSource(in, l, st, report)
		st.val.set(l, ownMoved)
		st.at.set(l, in.Node)
		a.noteMove(in, l, report)
	}
}

func (a *owner) moveGuardSource(in *midir.Inst, l midir.LocalID, st *dstate, report bool) {
	for up, ok := a.pl.parent[l]; ok; up, ok = a.pl.parent[l] {
		l = up
	}
	if src, ok := a.guardSources()[l]; ok {
		a.checkWhole(in.Node, src, st, report)
		a.checkDropOwner(in.Node, src, report)
		st.val.set(src, ownMoved)
		st.at.set(src, in.Node)
		a.noteMove(in, src, report)
	}
}

func ClearGuardMarks(f *midir.Func) {
	for bi := range f.Blocks {
		for i := range f.Blocks[bi].Insts {
			in := &f.Blocks[bi].Insts[i]
			if in.Op == midir.OpNop && in.Guarded {
				in.Args, in.Guarded, in.Str = nil, false, ""
			}
			if in.Op == midir.OpMove {
				in.Guarded = false
			}
		}
	}
}

// movesIntoBinding is a named binding taking a field place by value; the field read just before it already checked the place.
func (a *owner) movesIntoBinding(in *midir.Inst) bool {
	return in.Op == midir.OpCopy && !in.Guarded && a.f.Locals[in.Dst].Ent != 0 && a.tracked[in.Dst] &&
		a.pl.isPlace(in.Args[0]) && a.tracked[a.pl.canon(in.Args[0])]
}

func (a *owner) fieldMove(in *midir.Inst, l midir.LocalID, st *dstate, report bool) {
	a.checkDropOwner(in.Node, l, report)
	a.checkGuardHeld(in, l, st, report, 0)
	a.moveGuardSource(in, l, st, report)
	st.field.set(l, ownMoved)
}

func (a *owner) holdable(l midir.LocalID) bool {
	if a.pl.isPlace(l) {
		return true
	}
	loc := a.f.Locals[l]
	return loc.Ent != 0 && !loc.Borrowed && !loc.Cell
}

const GuardFailed = "guard-failed"
