package plan

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
)

// planAliases finds the alias temporaries (single-definition copies of a
// named local) that are still read after the local itself, or another
// alias of it, has been dropped: those must take their own reference when
// they are created instead of standing for the local.
func (p *planner) planAliases(f *midir.Func, isTemp func(midir.LocalID) bool, defs map[midir.LocalID]int) map[midir.LocalID]bool {
	origin := map[midir.LocalID]midir.LocalID{}
	defBlock := map[midir.LocalID]int{}
	defIdx := map[midir.LocalID]int{}
	for b, blk := range f.Blocks {
		for k, in := range blk.Insts {
			if in.Op != midir.OpCopy || !isTemp(in.Dst) || len(in.Args) != 1 {
				continue
			}
			src := in.Args[0]
			if o, ok := origin[src]; ok {
				src = o
			}
			if isTemp(src) {
				continue
			}
			origin[in.Dst] = src
			defBlock[in.Dst] = b
			defIdx[in.Dst] = k
		}
	}
	type site struct{ b, k int }
	dropsOf := map[midir.LocalID][]site{}
	usesOf := map[midir.LocalID][]site{}
	for b, blk := range f.Blocks {
		for k, in := range blk.Insts {
			if in.Op == midir.OpDrop && len(in.Args) == 1 {
				a := in.Args[0]
				dropsOf[a] = append(dropsOf[a], site{b, k})
				if o := origin[a]; o != a {
					dropsOf[o] = append(dropsOf[o], site{b, k})
				}
			}
			for _, a := range in.Args {
				if _, ok := origin[a]; ok {
					usesOf[a] = append(usesOf[a], site{b, k})
				}
			}
		}
		for _, a := range blk.Term.Args {
			if _, ok := origin[a]; ok {
				usesOf[a] = append(usesOf[a], site{b, len(blk.Insts)})
			}
		}
	}
	out := map[midir.LocalID]bool{}
	for t, o := range origin {
		if defs[t] != 1 {
			continue
		}
		dropped := false
		for _, d := range dropsOf[o] {
			if d.b != defBlock[t] || d.k > defIdx[t] {
				dropped = true
				break
			}
		}
		if !dropped {
			continue
		}
		for _, u := range usesOf[t] {
			if u.b != defBlock[t] || slices.ContainsFunc(dropsOf[o], func(d site) bool { return d.b == u.b && d.k > defIdx[t] && d.k < u.k }) {
				out[t] = true
				break
			}
		}
	}
	return out
}
