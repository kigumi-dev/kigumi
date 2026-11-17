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

// Another parameter assigned to it stays unknown: a relay names one parameter per call.
func (c *borrowCheck) assignedParamOrigins(g *midir.Func, defs map[midir.LocalID]*midir.Inst, i int) ([]calleeOrigin, bool) {
	p := g.Params[i]
	if g.Locals[p].Cell {
		return nil, false
	}
	out := []calleeOrigin{{param: i, isParam: true}}
	for _, d := range c.within(g).defsOf(p) {
		if d.Op != midir.OpCopy && d.Op != midir.OpMove || len(d.Args) == 0 || paramIndex(g, defs, d.Args[0]) >= 0 {
			return nil, false
		}
		origins, ok := c.calleeOrigins(g, defs, d.Args[0])
		if !ok || slices.ContainsFunc(origins, func(o calleeOrigin) bool { return o.isParam }) {
			return nil, false
		}
		out = append(out, origins...)
	}
	return out, true
}
