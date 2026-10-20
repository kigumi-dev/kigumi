package mir

import "sort"

// movedFields lists, in field order, root's own direct fields moved out of
// it on some path into here.
func (a *owner) movedFields(root LocalID, st *dstate) []int {
	var out []int
	for idx, rep := range a.pl.children[root] {
		if s := st.field[rep]; s == ownMoved || s == ownConflict {
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
		code, msg := partialMoveDiag(a.fieldName(root, fields[0]), a.ownerName(root))
		a.report(a.defNode[rep], code, msg)
	}
}

func (a *owner) useArgs(in *Inst, st *dstate, report bool) {
	if a.movesIntoBinding(in) {
		l := a.pl.canon(in.Args[0])
		a.checkWhole(in.Node, l, st, report)
		st.val[l], st.at[l] = ownMoved, in.Node
		return
	}
	for argi, arg := range in.Args {
		l := a.pl.canon(arg)
		if !a.tracked[l] {
			continue
		}
		a.check(in, l, st.val, report)
		switch {
		case a.pl.isPlace(arg) && (a.p.consumesArg(in, argi) || constructs(in.Op)):
			a.checkWhole(in.Node, l, st, report)
			st.val[l], st.at[l] = ownMoved, in.Node
		case in.Op == OpCall || in.Op == OpCallValue || in.Op == OpBuiltin || in.Op == OpBorrow:
			a.checkWhole(in.Node, l, st, report)
		}
	}
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
	if l := a.pl.canon(arg); a.pl.isPlace(arg) && a.tracked[l] {
		a.checkWhole(in.Node, l, st, report)
		st.val[l], st.at[l] = ownMoved, in.Node
	}
}

func clearGuardMarks(f *Func) {
	for bi := range f.Blocks {
		for i := range f.Blocks[bi].Insts {
			if in := &f.Blocks[bi].Insts[i]; in.Op == OpNop && in.Guarded {
				in.Args, in.Guarded = nil, false
			}
		}
	}
}

// movesIntoBinding is a named binding taking a field place by value; the field read just before it already checked the place.
func (a *owner) movesIntoBinding(in *Inst) bool {
	return in.Op == OpCopy && !in.Share && !in.Guarded && a.f.Locals[in.Dst].Ent != 0 && a.tracked[in.Dst] &&
		a.pl.isPlace(in.Args[0]) && a.tracked[a.pl.canon(in.Args[0])]
}
