package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// collectRefRead gives a value read below a reference what the referent holds at the read's path; a component holding
// a `&mut` is left alone, as a step towards a stored reference that heldReferents resolves.
func (c *borrowCheck) collectRefRead(in Inst) bool {
	t := c.f.Locals[in.Dst].Type
	if !c.projectsHeld(in) || c.typeHolds(t, func(t sem.TypeID) bool { _, mut := c.p.R.MutRefElem(t); return mut }) {
		return false
	}
	r, ok := c.readBase(in.Args[0], true)
	if !ok || !r.ref {
		return false
	}
	step, has := pathStep(&in, c.definition)
	b, ok := c.refReadContents(r.base, readHop{path: r.path.with(step), exact: r.exact && has})
	read, _ := c.readInfo(in.Args[0])
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
func (c *borrowCheck) refReadContents(r LocalID, hop readHop) (*borrowInfo, bool) {
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
		switch {
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
