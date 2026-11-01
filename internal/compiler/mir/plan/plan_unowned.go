package plan

import (
	"maps"

	midir "kigumi/internal/compiler/mir/ir"
)

// unownedTemps finds temporaries with no reference of their own, lent joins and their aliases included;
// each set iterates to a fixed point since block numbers do not follow definition order.
func (p *planner) unownedTemps(f *midir.Func, isTemp func(midir.LocalID) bool, defs map[midir.LocalID]int, materialize, joins map[midir.LocalID]bool, sharedAcross func(midir.LocalID) bool) map[midir.LocalID]bool {
	each := func(visit func(in midir.Inst) bool) {
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
	copies := func(in midir.Inst) bool {
		return (in.Op == midir.OpCopy || in.Op == midir.OpAlias) && len(in.Args) == 1
	}
	root := map[midir.LocalID]midir.LocalID{}
	rootOf := func(l midir.LocalID) (midir.LocalID, bool) {
		if !isTemp(l) {
			return l, true
		}
		r, ok := root[l]
		return r, ok
	}
	each(func(in midir.Inst) bool {
		if !copies(in) {
			return false
		}
		if _, done := root[in.Dst]; done {
			return false
		}
		r, ok := rootOf(in.Args[0])
		if !ok || !aliasesLocal(in, defs, materialize) || in.Op == midir.OpAlias && p.copyable(f.Locals[r].Type) {
			return false
		}
		root[in.Dst] = r
		return true
	})
	unowned := map[midir.LocalID]bool{}
	maps.Copy(unowned, joins)
	for l := range root {
		unowned[l] = true
	}
	each(func(in midir.Inst) bool {
		if unowned[in.Dst] {
			return false
		}
		switch {
		case in.Op == midir.OpBorrow, in.Op == midir.OpLendBox, in.Op == midir.OpBuiltin && in.Str == "iter.at_ref":
		case copies(in) && !(in.Share && defs[in.Dst] > 1) && isTemp(in.Args[0]) && !inRoots(root, in.Args[0]) && (unowned[in.Args[0]] || sharedAcross(in.Args[0])):
		default:
			return false
		}
		unowned[in.Dst] = true
		return true
	})
	return unowned
}

func inRoots(root map[midir.LocalID]midir.LocalID, l midir.LocalID) bool {
	_, ok := root[l]
	return ok
}
