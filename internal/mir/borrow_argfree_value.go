package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// valueKept reports, per argument of value call in after the callee, whether its result may keep it: what the
// keptSummary of every fn item the value can be says (calleeKeeps), and yes for any other callee.
func (c *borrowCheck) valueKept(in Inst) func(int) bool {
	ents, ok := c.fnItemOrigins(in.Args[0], 8)
	if !ok || in.Ent != 0 {
		return func(int) bool { return true }
	}
	return func(k int) bool {
		return slices.ContainsFunc(ents, func(e sem.EntityID) bool { return c.calleeKeeps(e, k, in.Args[1:]) })
	}
}

// calleeKeeps reports whether fn item e called with args may keep its argument k in its result: an async body's Future
// keeps them all, and a body without a keptSummary is taken to.
func (c *borrowCheck) calleeKeeps(e sem.EntityID, k int, args []LocalID) bool {
	if c.p.R.Entity(e).Kind != sem.EntFn || c.p.isAsyncBody(e) || k >= len(c.declaredParams(e)) {
		return true
	}
	s := c.keptBound(e, c.siteBind(e, args))
	return s == nil || s.keepsResult(k)
}

// fnItemOrigins are the fn items l holds, ok only when every value it can hold is one: through copies, joins, and a
// field, element or payload of a container built here.
func (c *borrowCheck) fnItemOrigins(l LocalID, depth int) ([]sem.EntityID, bool) {
	defs := c.defsOf(l)
	if depth == 0 || len(defs) == 0 || c.f.Locals[l].Cell {
		return nil, false
	}
	var out []sem.EntityID
	for _, d := range defs {
		var ents []sem.EntityID
		ok := false
		switch {
		case d.Op == OpFnItem:
			ents, ok = []sem.EntityID{d.Ent}, true
		case len(d.Args) == 0:
		case d.Op == OpCopy || d.Op == OpMove || d.Op == OpAlias:
			ents, ok = c.fnItemOrigins(d.Args[0], depth-1)
		case d.Op == OpField || d.Op == OpFieldMove || d.Op == OpPayload:
			ents, ok = c.fnItemParts(d.Args[0], d.Index, depth-1)
		case d.Op == OpIndex:
			ents, ok = c.fnItemParts(d.Args[0], -1, depth-1)
		}
		if !ok {
			return nil, false
		}
		out = append(out, ents...)
	}
	return out, true
}

// fnItemParts are the fn items part k of the container l (every part for -1) holds, l being built here and never
// written through.
func (c *borrowCheck) fnItemParts(l LocalID, k, depth int) ([]sem.EntityID, bool) {
	defs := c.defsOf(l)
	if depth == 0 || len(defs) == 0 || c.writtenThrough(l) {
		return nil, false
	}
	var out []sem.EntityID
	for _, d := range defs {
		parts := d.Args
		switch {
		case len(d.Args) == 0:
			return nil, false
		case d.Op == OpCopy || d.Op == OpMove || d.Op == OpAlias:
			ents, ok := c.fnItemParts(d.Args[0], k, depth-1)
			if !ok {
				return nil, false
			}
			out = append(out, ents...)
			continue
		case d.Op == OpCall && c.p.R.Entity(d.Ent).Name == "of" && c.p.R.Fn(d.Ent).Owner == c.p.R.Types.ArrayEnt():
			ents, ok := c.fnItemParts(d.Args[0], -1, depth-1)
			if !ok {
				return nil, false
			}
			out = append(out, ents...)
			continue
		case d.Op == OpRecord || d.Op == OpVariant || d.Op == OpArray:
			if k >= 0 && d.Op != OpArray {
				if k >= len(parts) {
					return nil, false
				}
				parts = parts[k : k+1]
			}
		default:
			return nil, false
		}
		for _, a := range parts {
			ents, ok := c.fnItemOrigins(a, depth-1)
			if !ok {
				return nil, false
			}
			out = append(out, ents...)
		}
	}
	return out, true
}

func (c *borrowCheck) defsOf(l LocalID) []*Inst {
	if c.defsAll == nil {
		c.defsAll = map[LocalID][]*Inst{}
		for b := range c.f.Blocks {
			for i := range c.f.Blocks[b].Insts {
				in := &c.f.Blocks[b].Insts[i]
				switch {
				case shapes[in.Op].result:
					c.defsAll[in.Dst] = append(c.defsAll[in.Dst], in)
				case in.Op == OpCellSet:
					c.defsAll[in.Args[0]] = append(c.defsAll[in.Args[0]], in)
				}
			}
		}
	}
	return c.defsAll[l]
}

// writtenThrough reports whether something stores into l or borrows it mutably.
func (c *borrowCheck) writtenThrough(l LocalID) bool {
	for _, blk := range c.f.Blocks {
		for _, in := range blk.Insts {
			if len(in.Args) > 0 && in.Args[0] == l && (in.Op == OpSetField || in.Op == OpSetIndex || in.Op == OpCellSet || in.Op == OpBorrow && in.Index == 1) {
				return true
			}
		}
	}
	return c.f.Locals[l].Cell
}
