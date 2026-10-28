package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// A caller's place gets none: checkFlow only validates stores into it.
func (c *borrowCheck) storeDests(fl storeFlow) []LocalID {
	out, _ := c.storeReach(fl)
	return out
}

// storeReach is where a flow's values land: this frame's locals, and the caller's places a local holder keeps a `&mut` of.
func (c *borrowCheck) storeReach(fl storeFlow) (out, callers []LocalID) {
	root, _, caller, ok := c.storeRoot(fl)
	if !ok || caller {
		return nil, nil
	}
	addCaller := func(o LocalID) {
		if !slices.Contains(callers, o) {
			callers = append(callers, o)
		}
	}
	add := func(o LocalID) {
		switch {
		case c.callerPlace(o):
			addCaller(o)
		case !slices.Contains(out, o) && c.p.R.Types.Kind(c.f.Locals[o].Type) != sem.KRef:
			out = append(out, o)
		}
	}
	holders := []LocalID{root}
	if !fl.byValue {
		out = append(out, root)
		for i, o := range c.referents(c.flowPlace(fl)) {
			if i > 0 {
				add(o.owner)
			}
		}
		holders = slices.Clone(out)
	}
	sites := c.calleeStoreSites(fl)
	if lit := c.closureLiteral(root); fl.byValue && lit != nil {
		if len(sites) > 0 {
			c.captureReach(lit, add, addCaller)
		}
		return out, callers
	}
	via := slices.ContainsFunc(sites, func(s paramStore) bool { return s.via })
	for _, d := range holders {
		if p, ok := c.heldOrigin(d); ok && via && c.keepsMutRef(c.f.Locals[d].Type) {
			addCaller(p)
		}
		for _, u := range c.siteOwners(d, sites) {
			if c.callerPlace(u) || c.keepsBorrows(u) {
				add(u)
			}
		}
	}
	return out, callers
}

// Only the `&mut`s d holds at the sites lead further; a shared borrow cannot be stored through.
func (c *borrowCheck) siteOwners(d LocalID, sites []paramStore) []LocalID {
	var out []LocalID
	for _, s := range sites {
		if !s.via {
			continue
		}
		for _, o := range c.heldAt(ownerRef{owner: d, exact: true}, c.holderOwners(d), readHop{path: s.path, exact: true}) {
			if o.mut && !slices.Contains(out, o.owner) {
				out = append(append(out, o.owner), c.heldMutOwners(o.owner)...)
			}
		}
	}
	return out
}

func (c *borrowCheck) keepsBorrows(l LocalID) bool {
	t := c.f.Locals[l].Type
	return c.elementHolder(t) || c.namesTypeParam(t) || c.holdsDeferred(t)
}

// valueSource follows the copies and moves of a temporary back to the local it came from.
func (c *borrowCheck) valueSource(l LocalID) LocalID {
	for range 8 {
		in := c.definition(l)
		if c.f.Locals[l].Ent != 0 || in == nil || len(in.Args) == 0 || in.Op != OpCopy && in.Op != OpMove && in.Op != OpAlias {
			break
		}
		l = in.Args[0]
	}
	return l
}

// For a call's `&mut` argument, flowPlace is the place it borrows.
func (c *borrowCheck) flowPlace(fl storeFlow) LocalID {
	l := fl.target
	for fl.ref && c.f.Locals[l].Ent == 0 {
		in := c.definition(l)
		if in == nil || len(in.Args) == 0 || in.Op != OpBorrow && in.Op != OpNewCell && in.Op != OpShare && in.Op != OpCopy && in.Op != OpMove && in.Op != OpAlias {
			return 0
		}
		l = in.Args[0]
		if in.Op == OpBorrow {
			break
		}
	}
	return l
}

// collectFlows widens each local target's sources with every value written
// into it; a caller's place is left to checkFlow, which only validates.
func (c *borrowCheck) collectFlows(in Inst) {
	for _, fl := range c.storeFlows(in) {
		c.collectFlow(fl, in)
	}
}

func (c *borrowCheck) collectFlow(fl storeFlow, in Inst) {
	c.callNode = in.Node
	dests := c.storeDests(fl)
	if len(dests) == 0 {
		return
	}
	infos := make([]*borrowInfo, len(fl.values))
	for i, v := range fl.values {
		if b, ok := c.storedInfo(fl, v, false); ok {
			infos[i] = c.lentContents(fl, v, b)
		}
	}
	for _, r := range dests {
		// A store through a `&mut` read out of a stored value finds its root only once holders are known.
		if !c.targets[r] {
			c.targets[r], c.grew = true, true
		}
		for i, v := range fl.values {
			b := infos[i]
			if b == nil || v.at != nil && !fl.byValue && r == dests[0] {
				continue
			}
			if v.tie != tieNone || fl.kept {
				b = c.tiedAt(fl, v.arg, r, b)
			} else if idx, ok := c.tieSite(fl.target, r); ok {
				c.retie(r, idx, v.arg, b)
			} else {
				b = untied(b)
			}
			c.merge(c.sink(r, in.Node), storedAt(c.fitDest(b, r)))
			if c.replacesHolder(in, r) {
				c.noteResetSite(r, b)
			} else if r == dests[0] {
				c.noteCallReset(in, fl, r, b)
			}
		}
	}
}

// fitDest is what of b r can keep: never a borrow of r itself (a widening artifact; checkFlow rejects the real case at
// the root), and when r's type is plain data holding no `&mut`, each borrow only as a shared one.
func (c *borrowCheck) fitDest(b *borrowInfo, r LocalID) *borrowInfo {
	plain := c.plainData(c.f.Locals[r].Type)
	if !slices.ContainsFunc(b.owners, func(o ownerRef) bool { return o.owner == r || plain && o.mut }) {
		return b
	}
	out := *b
	out.owners = nil
	for _, o := range b.owners {
		if o.owner != r {
			o.mut = o.mut && !plain
			out.owners = append(out.owners, o)
		}
	}
	return &out
}
