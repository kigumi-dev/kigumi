package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
)

// heldParam is the parameter of g whose field, element or payload the callable l is read out of, when nothing can
// write that part on the way: every local from the parameter to l is defined once and neither written nor lent mutably.
func (c *borrowCheck) heldParam(g *midir.Func, defs map[midir.LocalID]*midir.Inst, l midir.LocalID) (int, bool) {
	w := c.within(g)
	projected := false
	for range 16 {
		if w.writtenThrough(l) {
			return 0, false
		}
		if i := slices.Index(g.Params, l); i >= 0 {
			return i, projected && !c.assignedParam(g, l)
		}
		d := defs[l]
		if d == nil || len(d.Args) == 0 || g.Locals[l].Cell {
			return 0, false
		}
		switch {
		case d.Op == midir.OpCopy || d.Op == midir.OpMove || d.Op == midir.OpAlias:
		case projects(d):
			projected = true
		default:
			return 0, false
		}
		l = d.Args[0]
	}
	return 0, false
}

func projects(d *midir.Inst) bool {
	switch d.Op {
	case midir.OpField, midir.OpFieldMove, midir.OpIndex, midir.OpPayload:
		return true
	}
	return iterElement(d)
}

// heldCallees are the callables the value a of g holds: a callable's calleeOrigins, a parameter it is or is read out of
// as a held origin, else the fn items where a is built; false when they are unknown.
func (c *borrowCheck) heldCallees(g *midir.Func, defs map[midir.LocalID]*midir.Inst, a midir.LocalID) ([]calleeOrigin, bool) {
	if c.slotFnType(g.Locals[a].Type) != 0 {
		return c.calleeOrigins(g, defs, a)
	}
	if i := paramIndex(g, defs, a); i >= 0 {
		if c.assignedParam(g, g.Params[i]) || c.within(g).writtenThrough(g.Params[i]) {
			return nil, false
		}
		return []calleeOrigin{{param: i, isParam: true, held: true}}, true
	}
	if i, ok := c.heldParam(g, defs, a); ok {
		return []calleeOrigin{{param: i, isParam: true, held: true}}, true
	}
	items := c.within(g).fnItemsWithin(a, 8)
	if !items.known {
		return nil, false
	}
	var out []calleeOrigin
	for i, e := range items.ents {
		if !slices.Contains(items.ents[:i], e) {
			out = append(out, c.originOf(e))
		}
	}
	return out, true
}

// argOrigins are what parameter slot t of a call names as its callee: the callables the argument a holds when t is held.
func (c *borrowCheck) argOrigins(g *midir.Func, defs map[midir.LocalID]*midir.Inst, a midir.LocalID, held bool) ([]calleeOrigin, bool) {
	if held {
		return c.heldCallees(g, defs, a)
	}
	return c.calleeOrigins(g, defs, a)
}
