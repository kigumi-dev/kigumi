package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (c *borrowCheck) resolveKeptCallees(ents []sem.EntityID) []sem.EntityID {
	var out []sem.EntityID
	for _, e := range ents {
		if !c.ifaceRequirement(e) {
			out = append(out, e)
		} else if ws := c.implementers(e); len(ws) > 0 {
			out = append(out, ws...)
		} else {
			out = append(out, e)
		}
	}
	return out
}

func (c *borrowCheck) keptFlowCallees(in midir.Inst) ([]sem.EntityID, []midir.LocalID) {
	ents, args := []sem.EntityID{in.Ent}, in.Args
	if in.Op == midir.OpCallValue || in.Op == midir.OpFutureNew {
		args = in.Args[1:]
		if in.Ent == 0 {
			items := c.fnItemOrigins(in.Args[0], 8)
			if !items.known {
				return nil, args
			}
			ents = items.ents
		}
	}
	return c.resolveKeptCallees(ents), args
}

func (c *borrowCheck) summarizedValueCallees(in midir.Inst) []sem.EntityID {
	if in.Op != midir.OpCallValue || in.Ent != 0 {
		return nil
	}
	ents, _ := c.keptFlowCallees(in)
	if slices.ContainsFunc(ents, c.unsummarized) {
		return nil
	}
	return ents
}
