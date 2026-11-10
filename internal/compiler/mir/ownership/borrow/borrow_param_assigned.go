package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
)

// assignedParam reports a parameter of g its body assigns: its value at a use is then the caller's or any assignment's,
// so no definition alone names it.
func (c *borrowCheck) assignedParam(g *midir.Func, l midir.LocalID) bool {
	if !slices.Contains(g.Params, l) {
		return false
	}
	if c.state.assigned == nil {
		c.state.assigned = map[*midir.Func]map[midir.LocalID]bool{}
	}
	m, ok := c.state.assigned[g]
	if !ok {
		m = map[midir.LocalID]bool{}
		for _, blk := range g.Blocks {
			for _, in := range blk.Insts {
				switch {
				case in.Op.Yields() && slices.Contains(g.Params, in.Dst):
					m[in.Dst] = true
				case in.Op == midir.OpCellSet && slices.Contains(g.Params, in.Args[0]):
					m[in.Args[0]] = true
				}
			}
		}
		c.state.assigned[g] = m
	}
	return m[l] || g.Locals[l].Cell
}
