package mir

import "kigumi/internal/sem"

func (w *shareFacts) heldAcrossWrite(in Inst, i int) bool {
	if in.Op != OpCall && in.Op != OpCallValue || !w.writable(in.Args[i]) {
		return false
	}
	rs := w.rootsOf(in.Args[i])
	if len(rs.refs) == 0 && !rs.other {
		return false
	}
	for j, b := range in.Args {
		if j == i {
			continue
		}
		if w.lendsPlace(in, j) && meets(rs, w.rootsOf(b)) || w.closureWrites(b, rs) {
			return true
		}
	}
	return false
}

func (w *shareFacts) lendsPlace(in Inst, j int) bool {
	b := in.Args[j]
	if !w.passesPlace(b) && !w.mutRef(b) {
		return false
	}
	to, known := w.calleeParam(in, j)
	return !known || w.sum.writesThrough(to)
}

// A value of unknown origin may have captured a cell or any place this body only borrows, but nothing it owns.
func (w *shareFacts) closureWrites(b LocalID, rs rootSet) bool {
	return w.codeWrites(b, rs, map[LocalID]bool{})
}

func (w *shareFacts) codeWrites(v LocalID, rs rootSet, seen map[LocalID]bool) bool {
	if seen[v] || !w.runsCode(v) {
		return false
	}
	seen[v] = true
	if w.f.Locals[v].Ent != 0 {
		if len(w.binds[v]) == 0 || len(w.binds[v]) != len(w.sets[v]) {
			return w.unseenWrite(rs)
		}
		for _, at := range w.binds[v] {
			if w.codeWrites(w.inst(at).Args[0], rs, seen) {
				return true
			}
		}
		return false
	}
	if len(w.defs[v]) == 0 {
		return w.unseenWrite(rs)
	}
	for _, d := range w.defs[v] {
		switch {
		case d.Op == OpClosure:
			for _, c := range d.Args {
				if meets(rs, w.rootsOf(c)) || w.codeWrites(c, rs, seen) {
					return true
				}
			}
		case holdsFields(d.Op):
			for _, c := range d.Args {
				if w.codeWrites(c, rs, seen) {
					return true
				}
			}
		case passesOn(d.Op) && len(d.Args) == 1:
			if w.codeWrites(d.Args[0], rs, seen) {
				return true
			}
		case w.unseenWrite(rs):
			return true
		}
	}
	return false
}

func (w *shareFacts) unseenWrite(rs rootSet) bool {
	if rs.other {
		return true
	}
	for _, r := range rs.refs {
		if w.p.lentLocal(w.f, r.l) {
			return true
		}
	}
	return false
}

func (w *shareFacts) runsCode(v LocalID) bool {
	t := w.f.Locals[v].Type
	return w.p.R.Types.Kind(t) == sem.KParam || w.p.R.MayHoldFn(t)
}

func holdsFields(op Opcode) bool {
	switch op {
	case OpRecord, OpVariant, OpArray, OpBox, OpBind:
		return true
	}
	return false
}

func meets(a, b rootSet) bool {
	if a.other && b.other {
		return true
	}
	for _, x := range a.refs {
		for _, y := range b.refs {
			if x.l == y.l {
				return true
			}
		}
	}
	return false
}
