package mir

import (
	"sort"

	"kigumi/internal/sem"
)

func (a *owner) movedFields(root LocalID, st *dstate) []int {
	var out []int
	for idx, rep := range a.pl.children[root] {
		if gone(st.field[rep]) || a.tracked[rep] && gone(st.val[rep]) || len(a.movedFields(rep, st)) > 0 {
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

func (a *owner) useArgs(in *Inst, st *dstate, report bool) {
	a.cellStep(in, st, report)
	a.assignHeld(in, st, report)
	if in.Op == OpClosure {
		a.captureWhole(in, st, report)
	}
	if in.Op == OpCopy && in.Guarded {
		delete(st.rel, a.pl.canon(in.Args[0]))
	}
	if a.movesIntoBinding(in) {
		l := a.pl.canon(in.Args[0])
		a.checkWhole(in.Node, l, st, report)
		a.checkDropOwner(in.Node, l, report)
		a.checkGuardHeld(in, l, st, report && !a.fieldMoved(in.Args[0]), 0)
		a.moveGuardSource(in, l, st, report)
		if in.Share {
			// A shared scrutinee temporary keeps its own drop, so the binding takes the field without emptying it.
			st.field[l], st.at[l] = ownMoved, in.Node
		} else {
			st.val[l], st.at[l] = ownMoved, in.Node
		}
		a.noteMove(in, l, report && !a.fieldMoved(in.Args[0]))
		return
	}
	for argi, arg := range in.Args {
		l := a.pl.canon(arg)
		if in.Op == OpBorrow && in.Index == 1 || (in.Op == OpSetField || in.Op == OpSetIndex) && argi == 0 {
			a.checkGuardHeld(in, l, st, report, 0)
		}
		if !a.tracked[l] {
			continue
		}
		a.check(in, l, st.val, report)
		switch {
		case a.pl.isPlace(arg) && (a.p.consumesArg(in, argi) || a.p.primitiveTakes(in, argi) || constructs(in.Op)):
			a.checkWhole(in.Node, l, st, report)
			a.checkDropOwner(in.Node, l, report)
			a.checkGuardHeld(in, l, st, report && !a.fieldMoved(arg), 0)
			a.moveGuardSource(in, l, st, report)
			st.val[l], st.at[l] = ownMoved, in.Node
			a.noteMove(in, l, report)
		case in.Op == OpCall || in.Op == OpCallValue || in.Op == OpBuiltin || in.Op == OpBorrow || in.Op == OpTag || in.Op == OpIsVariant:
			a.checkWhole(in.Node, l, st, report)
		}
	}
}

// primitiveTakes: a runtime primitive retains what it keeps, so a field place it gets by value is spent like a moved local.
func (p *Program) primitiveTakes(in *Inst, i int) bool {
	if in.Op != OpCall || !p.IsStdPrimitive(in.Ent) {
		return false
	}
	recv := p.R.Fn(in.Ent).Recv
	return i != 0 || recv == sem.RecvNone || recv == sem.RecvMove
}

// constructs is an aggregate literal: a field place among its elements moves into it.
func constructs(op Opcode) bool {
	return op == OpRecord || op == OpVariant || op == OpArray
}

func (a *owner) guardedMove(in *Inst, st *dstate, report bool) {
	if !in.Guarded || len(in.Args) == 0 {
		return
	}
	arg := in.Args[0]
	if in.Str == guardFailed {
		st.rel[a.pl.canon(arg)] = ownValid
		return
	}
	a.cellConsume(in, arg, st, report)
	if l := a.pl.canon(arg); a.holdable(arg) && a.tracked[l] {
		a.checkWhole(in.Node, l, st, report)
		a.checkDropOwner(in.Node, l, report)
		var own LocalID
		if len(in.Args) > 1 {
			own = in.Args[1]
		}
		a.checkGuardHeld(in, l, st, report, own)
		a.moveGuardSource(in, l, st, report)
		st.val[l], st.at[l] = ownMoved, in.Node
		a.noteMove(in, l, report)
	}
}

func (a *owner) moveGuardSource(in *Inst, l LocalID, st *dstate, report bool) {
	for up, ok := a.pl.parent[l]; ok; up, ok = a.pl.parent[l] {
		l = up
	}
	if src, ok := a.guardSources()[l]; ok {
		a.checkWhole(in.Node, src, st, report)
		a.checkDropOwner(in.Node, src, report)
		st.val[src], st.at[src] = ownMoved, in.Node
		a.noteMove(in, src, report)
	}
}

func clearGuardMarks(f *Func) {
	for bi := range f.Blocks {
		for i := range f.Blocks[bi].Insts {
			in := &f.Blocks[bi].Insts[i]
			if in.Op == OpNop && in.Guarded {
				in.Args, in.Guarded, in.Str = nil, false, ""
			}
			if in.Op == OpMove {
				in.Guarded = false
			}
		}
	}
}

// movesIntoBinding is a named binding taking a field place by value; the field read just before it already checked the place.
func (a *owner) movesIntoBinding(in *Inst) bool {
	return in.Op == OpCopy && !in.Guarded && a.f.Locals[in.Dst].Ent != 0 && a.tracked[in.Dst] &&
		a.pl.isPlace(in.Args[0]) && a.tracked[a.pl.canon(in.Args[0])]
}

func (a *owner) fieldMove(in *Inst, l LocalID, st *dstate, report bool) {
	a.checkGuardHeld(in, l, st, report, 0)
	a.moveGuardSource(in, l, st, report)
	st.field[l] = ownMoved
}

func (a *owner) holdable(l LocalID) bool {
	if a.pl.isPlace(l) {
		return true
	}
	loc := a.f.Locals[l]
	return loc.Ent != 0 && !loc.Borrowed && !loc.Cell
}
