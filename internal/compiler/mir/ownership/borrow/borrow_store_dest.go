package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// A caller's place gets none: checkFlow only validates stores into it.
func (c *borrowCheck) storeDests(fl storeFlow) []midir.LocalID {
	out, _ := c.storeReach(fl)
	return out
}

// storeReach is where a flow's values land: this frame's locals, and the caller's places a local holder keeps a `&mut` of.
func (c *borrowCheck) storeReach(fl storeFlow) (out, callers []midir.LocalID) {
	root, _, caller, ok := c.storeRoot(fl)
	if !ok || caller {
		return nil, nil
	}
	addCaller := func(o midir.LocalID) {
		if !slices.Contains(callers, o) {
			callers = append(callers, o)
		}
	}
	add := func(o midir.LocalID) {
		switch {
		case c.callerPlace(o):
			addCaller(o)
		case !slices.Contains(out, o) && c.p.R.Types.Kind(c.f.Locals[o].Type) != sem.KRef:
			out = append(out, o)
		}
	}
	holders := []midir.LocalID{root}
	if !fl.byValue {
		out = append(out, root)
		if pl, ok := c.flowPlace(fl); ok {
			for i, o := range c.referents(pl) {
				if i > 0 {
					add(o.owner)
				}
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
func (c *borrowCheck) siteOwners(d midir.LocalID, sites []paramStore) []midir.LocalID {
	var out []midir.LocalID
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

func (c *borrowCheck) keepsBorrows(l midir.LocalID) bool {
	t := c.f.Locals[l].Type
	return c.elementHolder(t) || c.facts().NamesTypeParam(t) || c.holdsDeferred(t)
}

// valueSource follows the copies and moves of a temporary back to the local it came from.
func (c *borrowCheck) valueSource(l midir.LocalID) midir.LocalID {
	for range 8 {
		in := c.definition(l)
		if c.f.Locals[l].Ent != 0 || in == nil || len(in.Args) == 0 || in.Op != midir.OpCopy && in.Op != midir.OpMove && in.Op != midir.OpAlias {
			break
		}
		l = in.Args[0]
	}
	return l
}

// For a call's `&mut` argument, flowPlace is the place it borrows; not ok when the reference comes from elsewhere.
func (c *borrowCheck) flowPlace(fl storeFlow) (midir.LocalID, bool) {
	l := fl.target
	for fl.ref && c.f.Locals[l].Ent == 0 {
		in := c.definition(l)
		if in == nil || len(in.Args) == 0 || in.Op != midir.OpBorrow && in.Op != midir.OpNewCell && in.Op != midir.OpShare && in.Op != midir.OpCopy && in.Op != midir.OpMove && in.Op != midir.OpAlias {
			return 0, false
		}
		l = in.Args[0]
		if in.Op == midir.OpBorrow {
			break
		}
	}
	return l, true
}

// collectFlows widens each local target's sources with every value written
// into it; a caller's place is left to checkFlow, which only validates.
func (c *borrowCheck) collectFlows(in midir.Inst) {
	for _, fl := range c.storeFlows(in) {
		c.collectFlow(fl, in)
	}
}

func (c *borrowCheck) collectFlow(fl storeFlow, in midir.Inst) {
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
				// What a target reaches may hold ties this call makes, which start after its write-backs.
				if v.at != nil {
					idx = c.pastWriteBacks(idx, in.Args)
				}
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
func (c *borrowCheck) fitDest(b *borrowInfo, r midir.LocalID) *borrowInfo {
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
