package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// refPlaces are where a write into l's place lands when l is read through a `&mut` local or call result (`h.futs`,
// `fa.me().futs`): the places that reference borrows, extended by l's path.
func (c *borrowCheck) refPlaces(l midir.LocalID) []lentPlace {
	r, path, exact, ok := c.refBase(l)
	if !ok {
		return nil
	}
	b := c.throughRef(r, path, exact, true)
	if b == nil {
		return nil
	}
	return lentPlaces(c.heldReferents(r, b.owners), 0, r)
}

// referents resolves each holder a stored `&mut` was read through to what that holder keeps.
func (c *borrowCheck) referents(l midir.LocalID) []ownerRef {
	refs := c.storedReferents(l)
	if len(refs) == 0 {
		return nil
	}
	if d := c.definition(l); d != nil && len(d.Args) > 0 {
		if r, ok := c.readBase(d.Args[0], true); ok && !r.ref {
			if step, has := pathStep(d, c.definition); has {
				refs = c.heldAt(ownerRef{owner: r.base, exact: true}, refs, readHop{path: r.path.with(step), exact: r.exact})
			}
		}
	}
	return c.heldReferents(l, refs)
}

// heldReferents follows r back through each read of a field below another reference (`k.fb` with `k = &mut hr`):
// the holder of such a read lists what the outer reference points at, so each owner gives way to what it holds there.
func (c *borrowCheck) heldReferents(r midir.LocalID, owners []ownerRef) []ownerRef {
	var hops []readHop
	looped := false
	for seen := map[midir.LocalID]bool{}; ; {
		if looped = seen[r]; looped {
			break
		}
		seen[r] = true
		outer, hop, ok := c.outerRef(r)
		if !ok {
			break
		}
		hops, r = append(hops, hop), outer
	}
	for i := len(hops) - 1; i >= 0; i-- {
		var next []ownerRef
		for _, o := range c.derefOwners(owners) {
			h, ok := c.holders[o.owner]
			if !ok || !h.hasOwner || len(h.owners) == 0 {
				next = append(next, o)
				continue
			}
			for _, p := range c.heldAt(o, h.owners, hops[i]) {
				next = append(next, ownerRef{owner: p.owner, mut: true, path: p.path})
			}
		}
		owners = next
	}
	if !looped {
		return owners
	}
	// A chain of reads that loops back keeps every owner it could reach.
	for _, o := range slices.Clone(owners) {
		for _, u := range c.heldMutOwners(o.owner) {
			owners = append(owners, ownerRef{owner: u, mut: true})
		}
	}
	return owners
}

// derefOwners puts what a reference local points at, at the same path, in place of the local, as throughRef does.
func (c *borrowCheck) derefOwners(owners []ownerRef) []ownerRef {
	var out []ownerRef
	for _, o := range owners {
		h, ok := c.holders[o.owner]
		if c.p.R.Types.Kind(c.f.Locals[o.owner].Type) != sem.KRef || !ok || !h.hasOwner || len(h.owners) == 0 {
			out = append(out, o)
			continue
		}
		for _, p := range h.owners {
			path := append(slices.Clone(p.path), o.path...)
			out = append(out, ownerRef{owner: p.owner, mut: o.mut, path: path, exact: (len(p.path) == 0 || p.exact) && (len(o.path) == 0 || o.exact)})
		}
	}
	return out
}

// outerRef is the reference r was read out of a field below, through copies of the read, with the path of that read.
func (c *borrowCheck) outerRef(r midir.LocalID) (midir.LocalID, readHop, bool) {
	for seen := map[midir.LocalID]bool{}; !seen[r]; {
		seen[r] = true
		d := c.definition(r)
		if c.f.Locals[r].Ent != 0 {
			d = c.soleDef(r)
		}
		if d == nil || len(d.Args) == 0 {
			return 0, readHop{}, false
		}
		switch d.Op {
		case midir.OpCopy, midir.OpMove, midir.OpAlias:
			r = d.Args[0]
		case midir.OpField, midir.OpPayload, midir.OpIndex, midir.OpUnbox:
			base, hop, ok := c.readHopOf(d)
			return base, hop, ok && !slices.Contains(c.f.Params, base)
		default:
			return 0, readHop{}, false
		}
	}
	return 0, readHop{}, false
}

// heldMutOwners are the locals l keeps a `&mut` of, directly or through holders it keeps, so a callee lent l may
// write into them (`viaHR(&mut hr, ..)` pushing through `hr.fb`).
func (c *borrowCheck) heldMutOwners(l midir.LocalID) []midir.LocalID {
	var out []midir.LocalID
	seen := map[midir.LocalID]bool{l: true}
	for stack := []midir.LocalID{l}; len(stack) > 0; {
		u := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		h, ok := c.holders[u]
		if !ok || !h.hasOwner {
			continue
		}
		for _, o := range h.owners {
			if o.mut && !seen[o.owner] {
				seen[o.owner] = true
				out = append(out, o.owner)
				stack = append(stack, o.owner)
			}
		}
	}
	return out
}
