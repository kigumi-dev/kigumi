package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// emptyHandles is, per block and instruction, the handles known to hold nothing running there: on every path their
// content was moved out or its sole payload awaited, or a branch found them a variant whose payload keeps no handle.
func (c *borrowCheck) emptyHandles(handles map[LocalID]bool) [][]map[LocalID]bool {
	n := len(c.f.Blocks)
	in := make([]map[LocalID]bool, n)
	in[0] = map[LocalID]bool{}
	for changed := true; changed; {
		changed = false
		for _, b := range reversePostorder(c.f) {
			if in[b] == nil {
				continue
			}
			out := c.emptyThrough(b, copySet(in[b]), handles, nil)
			for k, t := range c.f.Blocks[b].Term.Targets {
				edge := c.emptyOnEdge(b, k, copySet(out), handles)
				if in[t] == nil {
					in[t], changed = edge, true
					continue
				}
				for l := range in[t] {
					if !edge[l] {
						delete(in[t], l)
						changed = true
					}
				}
			}
		}
	}
	at := make([][]map[LocalID]bool, n)
	for b := range c.f.Blocks {
		at[b] = make([]map[LocalID]bool, len(c.f.Blocks[b].Insts)+1)
		if in[b] != nil {
			end := c.emptyThrough(BlockID(b), copySet(in[b]), handles, func(i int, s map[LocalID]bool) { at[b][i] = copySet(s) })
			at[b][len(at[b])-1] = end
		}
	}
	return at
}

// emptyThrough applies a block to s, passing each instruction the handles empty just before it.
func (c *borrowCheck) emptyThrough(b BlockID, s, handles map[LocalID]bool, before func(int, map[LocalID]bool)) map[LocalID]bool {
	for i, in := range c.f.Blocks[b].Insts {
		if before != nil {
			before(i, s)
		}
		if in.Op == OpMove && handles[in.Args[0]] {
			s[in.Args[0]] = true
		}
		if in.Op == OpAwait {
			if o, ok := c.awaitedPayloadOf(in.Args[0], handles); ok {
				s[o] = true
			}
		}
		if in.Op.Yields() && definesDst(in) {
			delete(s, in.Dst)
		}
	}
	return s
}

// emptyOnEdge marks a handle a branch on its variant leaves holding nothing able to keep a Future or Task along target k.
func (c *borrowCheck) emptyOnEdge(b BlockID, k int, s, handles map[LocalID]bool) map[LocalID]bool {
	blk := &c.f.Blocks[b]
	if blk.Term.Op != TermBranch || len(blk.Term.Args) == 0 {
		return s
	}
	var test *Inst
	for i := range blk.Insts {
		in := &blk.Insts[i]
		switch {
		case !in.Op.Yields():
		case in.Dst == blk.Term.Args[0]:
			test = nil
			if in.Op == OpIsVariant && handles[in.Args[0]] {
				test = in
			}
		case test != nil && in.Dst == test.Args[0]:
			test = nil
		}
	}
	if test == nil {
		return s
	}
	payloadless := func(v sem.EntityID) bool { return !c.variantHoldsHandle(c.f.Locals[test.Args[0]].Type, v) }
	others := true
	for _, v := range c.p.R.TypeDecl(c.p.R.Entity(test.Ent).Parent).Variants {
		others = others && (v == test.Ent || payloadless(v))
	}
	if k == 0 && payloadless(test.Ent) || k == 1 && others {
		s[test.Args[0]] = true
	}
	return s
}

// awaitedPayloadOf is the handle an await of l finishes, l being through moves and copies the sole payload taken out
// of it; a taken payload only shares the handle's work, and a parameter may still hold its caller's value.
func (c *borrowCheck) awaitedPayloadOf(l LocalID, handles map[LocalID]bool) (LocalID, bool) {
	for range len(c.f.Locals) {
		d := c.soleDef(l)
		switch {
		case d == nil || len(d.Args) == 0 || slices.Contains(c.f.Params, l):
			return 0, false
		case d.Op == OpPayload && handles[d.Args[0]] && c.onePayloadEach(d.Args[0]):
			return d.Args[0], true
		case d.Op != OpMove && d.Op != OpCopy && d.Op != OpAlias:
			return 0, false
		}
		l = d.Args[0]
	}
	return 0, false
}

// onePayloadEach reports a value whose variants carry at most one payload each, so moving one out leaves it empty.
func (c *borrowCheck) onePayloadEach(l LocalID) bool {
	n := c.p.R.Types.Node(c.f.Locals[l].Type)
	if n.Kind != sem.KNamed || c.p.R.Entity(n.Ent).Kind != sem.EntType || c.p.R.TypeDecl(n.Ent).Form != sem.FormAdt {
		return false
	}
	for _, v := range c.p.R.TypeDecl(n.Ent).Variants {
		if len(c.p.R.Variant(v).Payload) > 1 {
			return false
		}
	}
	return true
}

// variantHoldsHandle reports whether variant v of t carries a payload, as t instantiates it, able to hold a Future or Task.
func (c *borrowCheck) variantHoldsHandle(t sem.TypeID, v sem.EntityID) bool {
	n := c.p.R.Types.Node(t)
	sub := map[sem.EntityID]sem.TypeID{}
	for i, p := range c.p.R.TypeDecl(c.p.R.Entity(v).Parent).Params {
		if i < len(n.Args) {
			sub[p] = n.Args[i]
		}
	}
	for _, pt := range c.p.R.Variant(v).Payload {
		if c.holdsHandle(c.p.R.Types.Subst(pt, sub)) {
			return true
		}
	}
	return false
}
