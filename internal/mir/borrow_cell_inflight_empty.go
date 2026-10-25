package mir

import "kigumi/internal/sem"

// emptyHandles is, per block and instruction, the handles known to hold nothing there: on every path in their content
// was moved out, or a branch found them a variant without a payload.
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
		if c.takesPayload(in) {
			if d := c.definition(in.Args[0]); d != nil && d.Op == OpPayload && handles[d.Args[0]] && c.onePayloadEach(d.Args[0]) {
				s[d.Args[0]] = true
			}
		}
		if in.Op.Yields() && definesDst(in) {
			delete(s, in.Dst)
		}
	}
	return s
}

// emptyOnEdge marks a handle a branch on its variant leaves holding no payload along target k.
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
	payloadless := func(v sem.EntityID) bool { return len(c.p.R.Variant(v).Payload) == 0 }
	others := true
	for _, v := range c.p.R.TypeDecl(c.p.R.Entity(test.Ent).Parent).Variants {
		others = others && (v == test.Ent || payloadless(v))
	}
	if k == 0 && payloadless(test.Ent) || k == 1 && others {
		s[test.Args[0]] = true
	}
	return s
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
