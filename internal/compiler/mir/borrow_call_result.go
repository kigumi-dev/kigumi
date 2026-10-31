package mir

import (
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// referentBorrow is what a call result tied to a borrows when a is a named reference (a parameter, binding or
// loop variable): a reborrow of a's referent, keeping a and what a borrows borrowed.
func (c *borrowCheck) referentBorrow(a LocalID, b *borrowInfo, recv sem.RecvKind) (*borrowInfo, bool) {
	t := c.f.Locals[a].Type
	if b.empty || b.fromStatic || b.multi != 0 || c.p.R.Types.Kind(t) != sem.KRef {
		return nil, false
	}
	if root, path, exact, ok := c.walkPlace(a, false); !ok || len(path) > 0 || !exact || c.p.R.Types.Kind(c.f.Locals[root].Type) != sem.KRef {
		return nil, false
	}
	mut := c.p.R.Types.RefMut(t)
	if recv != sem.RecvNone {
		mut = recv == sem.RecvMut
	}
	return c.receiverPlaceInfo(a, mut)
}

// tiedInfo is what cand's argument lends a call result: a reborrow of a named reference, else its own borrows.
func (c *borrowCheck) tiedInfo(args []LocalID, cand lifetimeCandidate) (*borrowInfo, bool) {
	b, ok := c.candidateInfo(args, cand)
	if !ok || cand.contents {
		return b, ok
	}
	if r, ok := c.referentBorrow(args[cand.arg], b, cand.recv); ok {
		return r, true
	}
	return b, true
}

// noteMixed remembers, for this call, a local of this function that one of owned borrows, when another candidate
// lends a parameter's borrow.
func (c *borrowCheck) noteMixed(owned []*borrowInfo) {
	for _, b := range owned {
		for _, o := range b.owners {
			if o.lifetime == 0 && !o.through && c.f.Locals[o.owner].Ent != 0 && !c.callerStorage(o.owner) {
				if c.mixed == nil {
					c.mixed = map[syntax.NodeID]LocalID{}
				}
				c.mixed[c.callNode] = o.owner
				return
			}
		}
	}
}

func (c *borrowCheck) reportUntrackedCall(in Inst) {
	if l, ok := c.mixed[in.Node]; ok {
		c.reportCode(in.Node, in.Dst, "lifetime-mixed-call", c.f.Locals[l].Name)
		return
	}
	code, msg := lifetimeUntrackedDiag("lifetime-untracked-call")
	c.report(in.Node, in.Dst, code, msg)
}

// unionSite is a merge of several candidates' borrows, which takes in what each of them gains later.
type unionSite struct {
	info  *borrowInfo
	parts []*borrowInfo
}

// reborrowSite is a cached borrow of a place under root, which takes in what root borrows as that grows.
type reborrowSite struct {
	info  *borrowInfo
	root  LocalID
	path  stepPath
	exact bool
	mut   bool
}

func (s reborrowSite) refresh(c *borrowCheck) {
	if b := c.throughRef(s.root, s.path, s.exact, s.mut); b != nil {
		c.merge(s.info, storedAt(b))
	}
}

func (c *borrowCheck) refreshReborrows() {
	for _, s := range c.reborrows {
		s.refresh(c)
	}
	for _, u := range c.unions {
		for _, b := range u.parts {
			c.merge(u.info, b)
		}
	}
}

func (c *borrowCheck) referentOr(a LocalID, b *borrowInfo) *borrowInfo {
	if r, ok := c.referentBorrow(a, b, sem.RecvNone); ok {
		return r
	}
	return b
}

// fromCall reports whether reference l may hold a call's result, which points somewhere inside what its
// arguments reach rather than at their own place, so a path taken below l cannot extend theirs.
func (c *borrowCheck) fromCall(l LocalID) bool {
	if c.callRefs == nil {
		c.callRefs = map[LocalID]bool{}
		defs := map[LocalID][]*Inst{}
		for b := range c.f.Blocks {
			for i := range c.f.Blocks[b].Insts {
				if in := &c.f.Blocks[b].Insts[i]; shapes[in.Op].result {
					defs[in.Dst] = append(defs[in.Dst], in)
				}
			}
		}
		for changed := true; changed; {
			changed = false
			for dst, ds := range defs {
				if c.callRefs[dst] {
					continue
				}
				for _, d := range ds {
					if d.Op == OpCall || d.Op == OpCallValue || (d.Op == OpCopy || d.Op == OpMove || d.Op == OpAlias) && c.callRefs[d.Args[0]] {
						c.callRefs[dst], changed = true, true
						break
					}
				}
			}
		}
	}
	return c.callRefs[l]
}
