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

// leftBehind: argument i is a named local whose source slot was stored over since, so the local alone holds the old box.
func (w *shareFacts) leftBehind(in Inst, i int, at Point) bool {
	o, ok := w.namedOrigin(in.Args[i])
	if !ok || len(w.sets[o]) != 1 || len(w.binds[o]) != 1 || w.observed(o, at) {
		return false
	}
	bp := w.binds[o][0]
	r, s, ok := w.placeOf(w.inst(bp).Args[0])
	if !ok || len(s) == 0 || s[len(s)-1].op != OpField || len(w.sets[r]) > 1 || bp.Block != at.Block || bp.Inst >= at.Inst {
		return false
	}
	if rs := w.rootsOf(o); rs.other || len(rs.refs) != 2 {
		return false
	}
	for _, v := range w.viewersOf(r) {
		if v != o && w.liveness().after(v, at.Block, at.Inst) && !w.apart(v, r, s) {
			return false
		}
	}
	blk, over := w.blocks[at.Block], false
	for k := bp.Inst + 1; k < at.Inst; k++ {
		x := blk.Insts[k]
		if x.Op != OpSetField {
			if !w.leaves(x, r, s) {
				return false
			}
			continue
		}
		b, p, ok := w.placeOf(x.Args[0])
		t := append(slices.Clip(p), step{OpField, x.Index})
		switch {
		case !ok || b != r || !overlaps(t, wpath{s, true}):
		case !slices.Equal(t, s) || slices.ContainsFunc(w.rootsOf(x.Args[1]).refs, func(y rootRef) bool { return y.l == o }):
			return false
		default:
			over = true
		}
	}
	return over
}

func (w *shareFacts) namedOrigin(v LocalID) (LocalID, bool) {
	for range w.nLocals {
		if w.f.Locals[v].Ent != 0 {
			return v, true
		}
		ds := w.defs[v]
		if len(ds) != 1 || len(ds[0].Args) != 1 || ds[0].Op != OpAlias && (ds[0].Op != OpCopy || ds[0].Share) {
			return 0, false
		}
		v = ds[0].Args[0]
	}
	return 0, false
}
