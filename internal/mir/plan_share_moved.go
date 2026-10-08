package mir

import "slices"

func (w *shareFacts) movedOut(in Inst, i int, at Point) bool {
	a := in.Args[i]
	if in.Op != OpSetField || i != 1 || w.liveness().after(a, at.Block, at.Inst) {
		return false
	}
	r, s, ok := w.placeOf(a)
	if !ok || len(s) == 0 || s[len(s)-1].op != OpField || len(w.sets[r]) > 1 {
		return false
	}
	if rs := w.rootsOf(a); rs.other || len(rs.refs) != 1 {
		return false
	}
	for _, v := range w.viewersOf(r) {
		if v != a && w.liveness().after(v, at.Block, at.Inst) && !w.apart(v, r, s) {
			return false
		}
	}
	blk := w.blocks[at.Block]
	for k := at.Inst + 1; k < len(blk.Insts); k++ {
		x := blk.Insts[k]
		if x.Op == OpSetField {
			b, p, ok := w.placeOf(x.Args[0])
			t := append(slices.Clip(p), step{OpField, x.Index})
			switch {
			case slices.Contains(x.Args[1:], r):
				return false
			case !ok || b != r || !overlaps(t, wpath{s, true}):
				continue
			}
			return slices.Equal(t, s)
		}
		if !w.leaves(x, r, s) {
			return false
		}
	}
	return false
}

func (w *shareFacts) leaves(x Inst, r LocalID, s []step) bool {
	switch x.Op {
	case OpField:
		b, p, ok := w.placeOf(x.Dst)
		if !ok || b != r {
			return !slices.Contains(x.Args, r)
		}
		return !overlaps(p, wpath{s, true}) || len(p) < len(s) && w.onlyStoresInto(x.Dst)
	case OpCopy, OpAlias, OpMove, OpConst, OpUnit, OpDrop, OpRelease:
		return !slices.Contains(x.Args, r)
	}
	return false
}

func (w *shareFacts) apart(v, r LocalID, s []step) bool {
	srcs := []LocalID{v}
	if w.f.Locals[v].Ent != 0 {
		if len(w.sets[v]) != len(w.binds[v]) {
			return false
		}
		srcs = srcs[:0]
		for _, at := range w.binds[v] {
			srcs = append(srcs, w.inst(at).Args[0])
		}
	}
	for _, x := range srcs {
		if b, p, ok := w.placeOf(x); !ok || b != r || overlaps(p, wpath{s, true}) {
			return false
		}
	}
	return true
}

func (w *shareFacts) onlyStoresInto(d LocalID) bool {
	for _, u := range w.uses[d] {
		if u.term {
			return false
		}
		if x := w.inst(u.at); (x.Op != OpSetField || u.i != 0) && x.Op != OpDrop && x.Op != OpRelease {
			return false
		}
	}
	return true
}
