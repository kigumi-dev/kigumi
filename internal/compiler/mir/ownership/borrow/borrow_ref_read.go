package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// collectRefRead gives a value read below a reference what the referent holds at the read's path, and the reference's
// own borrow only if the value could be such a reference; a component holding a `&mut` is left to heldReferents.
func (c *borrowCheck) collectRefRead(in midir.Inst) bool {
	t := c.f.Locals[in.Dst].Type
	if !c.projectsHeld(in) || c.mutRefInside(t) {
		return false
	}
	r, ok := c.readBase(in.Args[0], true)
	if !ok || !r.ref {
		return false
	}
	step, has := ownquery.PathStep(&in, c.definition)
	b, ok := c.refReadContents(r.base, readHop{path: r.path.With(step), exact: r.exact && has})
	read, _ := c.readInfo(in.Args[0])
	if known := ok && (b.empty || b.hasOwner || b.lifetime != 0 || b.fromStatic); known && !c.mayHoldRef(t, c.f.Locals[r.base].Type) {
		read = nil
	}
	_, sunk := c.sinks[in.Dst]
	if !sunk && (!ok || read != nil && !slices.ContainsFunc(b.owners, func(o ownerRef) bool { return !slices.ContainsFunc(read.owners, o.same) })) {
		return false
	}
	s := c.sink(in.Dst, in.Node)
	if read != nil {
		c.merge(s, read)
	}
	if ok {
		c.merge(s, b)
	}
	return true
}

// refReadContents is what the places r points at hold under hop, with what their `&mut`s lead to.
func (c *borrowCheck) refReadContents(r midir.LocalID, hop readHop) (*borrowInfo, bool) {
	if b, ok := c.paramContents(r); ok {
		return b, true
	}
	pointees := c.referents(r)
	if len(pointees) == 0 {
		h, ok := c.holders[r]
		if !ok || !h.hasOwner {
			return nil, false
		}
		pointees = h.owners
	}
	out := &borrowInfo{empty: true}
	for _, p := range c.derefOwners(pointees) {
		h, ok := c.holders[p.owner]
		pc, param := c.paramContents(p.owner)
		switch {
		case param:
			mergeInto(out, pc)
		case !ok:
		case !h.hasOwner:
			mergeInto(out, h)
		default:
			mergeInto(out, &borrowInfo{owners: c.heldAt(p, h.owners, hop), hasOwner: true, node: h.node})
		}
	}
	if out.empty || !out.hasOwner {
		return out, !out.empty
	}
	return c.withHeld(out), true
}

// mayHoldRef reports whether a value of t could keep a reference like ref: one to the same type, or a type parameter.
func (c *borrowCheck) mayHoldRef(t, ref sem.TypeID) bool {
	elem := c.p.R.Types.Node(ref).Elem
	return c.facts().NamesTypeParam(t) || c.typeHolds(t, func(u sem.TypeID) bool {
		return c.p.R.Types.Kind(u) == sem.KRef && c.p.R.Types.Node(u).Elem == elem
	})
}
