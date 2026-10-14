package mir

// unownedTemps finds temporaries with no reference of their own; each set iterates to a fixed point since block numbers do not follow definition order.
func (p *Program) unownedTemps(f *Func, isTemp func(LocalID) bool, defs map[LocalID]int, materialize map[LocalID]bool, sharedAcross func(LocalID) bool) map[LocalID]bool {
	each := func(visit func(in Inst) bool) {
		for changed := true; changed; {
			changed = false
			for _, blk := range f.Blocks {
				for _, in := range blk.Insts {
					if definesTemp(in, isTemp) && visit(in) {
						changed = true
					}
				}
			}
		}
	}
	copies := func(in Inst) bool { return (in.Op == OpCopy || in.Op == OpAlias) && len(in.Args) == 1 }
	root := map[LocalID]LocalID{}
	rootOf := func(l LocalID) (LocalID, bool) {
		if !isTemp(l) {
			return l, true
		}
		r, ok := root[l]
		return r, ok
	}
	each(func(in Inst) bool {
		if !copies(in) {
			return false
		}
		if _, done := root[in.Dst]; done {
			return false
		}
		r, ok := rootOf(in.Args[0])
		if !ok || !aliasesLocal(in, defs, materialize) || in.Op == OpAlias && p.copyable(f.Locals[r].Type) {
			return false
		}
		root[in.Dst] = r
		return true
	})
	unowned := map[LocalID]bool{}
	for l := range root {
		unowned[l] = true
	}
	each(func(in Inst) bool {
		if unowned[in.Dst] {
			return false
		}
		switch {
		case in.Op == OpBorrow, in.Op == OpBuiltin && in.Str == "iter.at_ref":
		case copies(in) && isTemp(in.Args[0]) && !inRoots(root, in.Args[0]) && (unowned[in.Args[0]] || sharedAcross(in.Args[0])):
		default:
			return false
		}
		unowned[in.Dst] = true
		return true
	})
	return unowned
}

func inRoots(root map[LocalID]LocalID, l LocalID) bool {
	_, ok := root[l]
	return ok
}
