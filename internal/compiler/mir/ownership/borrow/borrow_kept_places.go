package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// refRoots are the locals the reference or cell a names, through borrows, cells, projections and copies of a reference.
func (c *borrowCheck) refRoots(g *midir.Func, defs map[midir.LocalID][]*midir.Inst, a midir.LocalID) []midir.LocalID {
	out := []midir.LocalID{a}
	for i := 0; i < len(out); i++ {
		for _, d := range defs[out[i]] {
			switch d.Op {
			case midir.OpBorrow, midir.OpShare, midir.OpNewCell, midir.OpLendBox, midir.OpCellGet, midir.OpField, midir.OpFieldMove, midir.OpIndex, midir.OpPayload, midir.OpAlias:
			case midir.OpCopy, midir.OpMove:
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
func borrowedPlaces(g *midir.Func, defs map[midir.LocalID][]*midir.Inst) map[midir.LocalID]bool {
	out := map[midir.LocalID]bool{}
	var mark func(midir.LocalID)
	mark = func(l midir.LocalID) {
		if out[l] {
			return
		}
		out[l] = true
		for _, d := range defs[l] {
			switch d.Op {
			case midir.OpField, midir.OpFieldMove, midir.OpIndex, midir.OpPayload, midir.OpCellGet, midir.OpAlias:
				mark(d.Args[0])
			}
		}
	}
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpBorrow {
				mark(in.Args[0])
			}
		}
	}
	return out
}
