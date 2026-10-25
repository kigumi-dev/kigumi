package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// refPlaces are where a write into l's place lands when l is read through a `&mut` local or call result (`h.futs`,
// `fa.me().futs`): the places that reference borrows, extended by l's path.
func (c *borrowCheck) refPlaces(l LocalID) []lentPlace {
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

// refBase walks l's field, payload and index reads down to the reference they read through, with l's path below it.
func (c *borrowCheck) refBase(l LocalID) (LocalID, []int, bool, bool) {
	path, exact := []int(nil), true
	for seen := map[LocalID]bool{}; !seen[l]; {
		seen[l] = true
		if c.p.R.Types.Kind(c.f.Locals[l].Type) == sem.KRef {
			return l, path, exact, !slices.Contains(c.f.Params, l)
		}
		in := c.definition(l)
		if c.f.Locals[l].Ent != 0 || in == nil || len(in.Args) == 0 {
			return 0, nil, false, false
		}
		switch in.Op {
		case OpField:
			path = append([]int{in.Index}, path...)
		case OpPayload:
			path = append([]int{payloadStep(in.Index)}, path...)
		case OpIndex, OpUnbox:
			path, exact = nil, false
		case OpCopy:
		default:
			return 0, nil, false, false
		}
		l = in.Args[0]
	}
	return 0, nil, false, false
}

// heldReferents follows r back through each read of a field below another reference (`k.fb` with `k = &mut hr`):
// the holder of such a read lists what the outer reference points at, so each owner gives way to the borrows it holds.
func (c *borrowCheck) heldReferents(r LocalID, owners []ownerRef) []ownerRef {
	for seen := map[LocalID]bool{}; !seen[r]; {
		seen[r] = true
		outer, ok := c.outerRef(r)
		if !ok {
			return owners
		}
		var next []ownerRef
		for _, o := range owners {
			h, ok := c.holders[o.owner]
			if !ok || !h.hasOwner || len(h.owners) == 0 {
				next = append(next, o)
				continue
			}
			for _, p := range h.owners {
				next = append(next, ownerRef{owner: p.owner, mut: true, path: p.path})
			}
		}
		owners, r = next, outer
	}
	// A chain of reads that loops back keeps every owner it could reach.
	for _, o := range slices.Clone(owners) {
		for _, u := range c.heldMutOwners(o.owner) {
			owners = append(owners, ownerRef{owner: u, mut: true})
		}
	}
	return owners
}

// outerRef is the reference r was read out of a field below, through copies of the read.
func (c *borrowCheck) outerRef(r LocalID) (LocalID, bool) {
	for seen := map[LocalID]bool{}; !seen[r]; {
		seen[r] = true
		d := c.definition(r)
		if c.f.Locals[r].Ent != 0 {
			d = soleDef(c.f, r)
		}
		if d == nil || len(d.Args) == 0 {
			return 0, false
		}
		switch d.Op {
		case OpCopy, OpMove, OpAlias:
			r = d.Args[0]
		case OpField, OpPayload, OpIndex, OpUnbox:
			base, _, _, ok := c.refBase(d.Args[0])
			return base, ok
		default:
			return 0, false
		}
	}
	return 0, false
}

// heldMutOwners are the locals l keeps a `&mut` of, directly or through holders it keeps, so a callee lent l may
// write into them (`viaHR(&mut hr, ..)` pushing through `hr.fb`).
func (c *borrowCheck) heldMutOwners(l LocalID) []LocalID {
	var out []LocalID
	seen := map[LocalID]bool{l: true}
	for stack := []LocalID{l}; len(stack) > 0; {
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
