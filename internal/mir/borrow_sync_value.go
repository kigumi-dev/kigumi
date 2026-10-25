package mir

import "kigumi/internal/sem"

// syncFnValue reports whether every value l can hold is a synchronous fn item, whose Future is built by the callee
// and cannot keep its `&mut`: through copies, joins, and a field, element or payload of a container built here.
func (c *borrowCheck) syncFnValue(l LocalID, depth int) bool {
	defs := c.defsOf(l)
	if depth == 0 || len(defs) == 0 {
		return false
	}
	for _, d := range defs {
		switch d.Op {
		case OpFnItem:
			if c.p.R.Entity(d.Ent).Kind != sem.EntFn || c.p.isAsyncBody(d.Ent) {
				return false
			}
		case OpCopy, OpMove, OpAlias:
			if len(d.Args) == 0 || !c.syncFnValue(d.Args[0], depth-1) {
				return false
			}
		case OpField, OpFieldMove, OpPayload:
			if len(d.Args) == 0 || !c.syncParts(d.Args[0], d.Index, depth-1) {
				return false
			}
		case OpIndex:
			if len(d.Args) == 0 || !c.syncParts(d.Args[0], -1, depth-1) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// syncParts: part k of the container l (every part for -1) is a synchronous fn item, l being built here and never
// written through.
func (c *borrowCheck) syncParts(l LocalID, k, depth int) bool {
	defs := c.defsOf(l)
	if depth == 0 || len(defs) == 0 || c.writtenThrough(l) {
		return false
	}
	for _, d := range defs {
		parts := d.Args
		switch {
		case len(d.Args) == 0:
			return false
		case d.Op == OpCopy || d.Op == OpMove || d.Op == OpAlias:
			if !c.syncParts(d.Args[0], k, depth-1) {
				return false
			}
			continue
		case d.Op == OpCall && c.p.R.Entity(d.Ent).Name == "of" && c.p.R.Fn(d.Ent).Owner == c.p.R.Types.ArrayEnt():
			if !c.syncParts(d.Args[0], -1, depth-1) {
				return false
			}
			continue
		case d.Op == OpRecord || d.Op == OpVariant || d.Op == OpArray:
			if k >= 0 && d.Op != OpArray {
				if k >= len(parts) {
					return false
				}
				parts = parts[k : k+1]
			}
		default:
			return false
		}
		for _, a := range parts {
			if !c.syncFnValue(a, depth-1) {
				return false
			}
		}
	}
	return true
}

func (c *borrowCheck) defsOf(l LocalID) []*Inst {
	var out []*Inst
	for b := range c.f.Blocks {
		for i := range c.f.Blocks[b].Insts {
			if in := &c.f.Blocks[b].Insts[i]; shapes[in.Op].result && in.Dst == l {
				out = append(out, in)
			}
		}
	}
	return out
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
