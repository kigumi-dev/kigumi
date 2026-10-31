package mir

import (
	"slices"

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

func (c *borrowCheck) keptFlowCallees(in Inst) ([]sem.EntityID, []LocalID) {
	ents, args := []sem.EntityID{in.Ent}, in.Args
	if in.Op == OpCallValue || in.Op == OpFutureNew {
		args = in.Args[1:]
		if in.Ent == 0 {
			var ok bool
			if ents, ok = c.fnItemOrigins(in.Args[0], 8); !ok {
				return nil, args
			}
		}
	}
	return c.resolveKeptCallees(ents), args
}

func (c *borrowCheck) summarizedValueCallees(in Inst) []sem.EntityID {
	if in.Op != OpCallValue || in.Ent != 0 {
		return nil
	}
	ents, _ := c.keptFlowCallees(in)
	if slices.ContainsFunc(ents, c.unsummarized) {
		return nil
	}
	return ents
}
