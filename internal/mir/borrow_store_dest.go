package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// A caller's place gets none: checkFlow only validates stores into it.
func (c *borrowCheck) storeDests(fl storeFlow) []LocalID {
	root, _, caller, ok := c.storeRoot(fl)
	if !ok || caller {
		return nil
	}
	out := []LocalID{root}
	add := func(o LocalID) {
		if !slices.Contains(out, o) && !c.callerPlace(o) && c.p.R.Types.Kind(c.f.Locals[o].Type) != sem.KRef {
			out = append(out, o)
		}
	}
	for i, o := range c.referents(c.flowPlace(fl)) {
		if i > 0 {
			add(o.owner)
		}
	}
	sites := c.calleeStoreSites(fl)
	for _, d := range slices.Clone(out) {
		for _, u := range c.siteOwners(d, sites) {
			if c.keepsBorrows(u) {
				add(u)
			}
		}
	}
	return out
}

// Only the `&mut`s d holds at the sites lead further; a shared borrow cannot be stored through.
func (c *borrowCheck) siteOwners(d LocalID, sites []paramStore) []LocalID {
	var out []LocalID
	for _, s := range sites {
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

// A callee without a body may store only when the call hands it a value that carries a borrow.
func (c *borrowCheck) calleeStoreSites(fl storeFlow) []paramStore {
	if fl.callee == 0 && !fl.opaque {
		return nil
	}
	if g := c.p.ByEnt[fl.callee]; g != nil && !fl.opaque {
		return c.paramStoreSites(g, fl.param)
	}
	if slices.ContainsFunc(fl.values, func(v flowArg) bool { return c.carries(c.f.Locals[v.arg].Type) }) {
		return []paramStore{{}}
	}
	return nil
}

// lentContents drops the exclusive borrows from another `&mut` argument's contents when the callee never stores
// through that parameter, since only such a store can move them.
func (c *borrowCheck) lentContents(fl storeFlow, v flowArg, b *borrowInfo) *borrowInfo {
	if !v.contents || fl.callee == 0 || !b.hasOwner || len(c.calleeStoreSites(storeFlow{callee: fl.callee, param: v.param, values: fl.values})) > 0 {
		return b
	}
	out := *b
	out.owners = slices.DeleteFunc(slices.Clone(b.owners), func(o ownerRef) bool { return o.mut })
	return &out
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
			if b == nil {
				continue
			}
			if v.tie != tieNone {
				b = c.tiedAt(fl.target, r, b)
			} else if idx, ok := c.tieSite(fl.target, r); ok {
				c.retie(r, idx, v.arg, b)
			} else {
				b = untied(b)
			}
			c.merge(c.sink(r, in.Node), storedAt(c.fitDest(b, r)))
			if c.replacesHolder(in, r) {
				c.noteResetSite(r, b)
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

// plainData reports whether a value of t holds no `&mut`, closure, Future or type parameter, so it can keep a
// borrow only as a shared one.
func (c *borrowCheck) plainData(t sem.TypeID) bool {
	plain, ok := c.plainTypes[t]
	if !ok {
		plain = !c.namesTypeParam(t) && !c.holdsDeferred(t) && !c.typeHolds(t, func(t sem.TypeID) bool { _, mut := c.p.R.MutRefElem(t); return mut })
		if c.plainTypes == nil {
			c.plainTypes = map[sem.TypeID]bool{}
		}
		c.plainTypes[t] = plain
	}
	return plain
}
