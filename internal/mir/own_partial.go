package mir

import "sort"

// movedFields lists, in field order, root's own direct fields currently
// moved out of it.
func (a *owner) movedFields(root LocalID, st *dstate) []int {
	var out []int
	for idx, rep := range a.pl.children[root] {
		if st.field[rep] == ownMoved {
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
		if a.pl.isPlace(arg) && a.p.consumesArg(in, argi) {
			a.checkWhole(in.Node, l, st, report)
			st.val[l], st.at[l] = ownMoved, in.Node
		}
	}
}

// movesIntoBinding is a named binding taking a field place by value; the field read just before it already checked the place.
func (a *owner) movesIntoBinding(in *Inst) bool {
	return in.Op == OpCopy && !in.Share && a.f.Locals[in.Dst].Ent != 0 && a.tracked[in.Dst] &&
		a.pl.isPlace(in.Args[0]) && a.tracked[a.pl.canon(in.Args[0])]
}
