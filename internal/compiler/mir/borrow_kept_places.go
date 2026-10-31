package mir

import (
	"slices"

	"kigumi/internal/compiler/sem"
)

// refRoots are the locals the reference or cell a names, through borrows, cells, projections and copies of a reference.
func (c *borrowCheck) refRoots(g *Func, defs map[LocalID][]*Inst, a LocalID) []LocalID {
	out := []LocalID{a}
	for i := 0; i < len(out); i++ {
		for _, d := range defs[out[i]] {
			switch d.Op {
			case OpBorrow, OpShare, OpNewCell, OpLendBox, OpCellGet, OpField, OpFieldMove, OpIndex, OpPayload, OpAlias:
			case OpCopy, OpMove:
				if c.p.R.Types.Kind(g.Locals[out[i]].Type) != sem.KRef {
					continue
				}
			default:
				continue
			}
			if len(d.Args) > 0 && !slices.Contains(out, d.Args[0]) {
				out = append(out, d.Args[0])
			}
		}
	}
	return out
}

// borrowedPlaces are the locals a borrow is taken of, followed back through the projections and cell reads naming them.
func borrowedPlaces(g *Func, defs map[LocalID][]*Inst) map[LocalID]bool {
	out := map[LocalID]bool{}
	var mark func(LocalID)
	mark = func(l LocalID) {
		if out[l] {
			return
		}
		out[l] = true
		for _, d := range defs[l] {
			switch d.Op {
			case OpField, OpFieldMove, OpIndex, OpPayload, OpCellGet, OpAlias:
				mark(d.Args[0])
			}
		}
	}
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			if in.Op == OpBorrow {
				mark(in.Args[0])
			}
		}
	}
	return out
}
