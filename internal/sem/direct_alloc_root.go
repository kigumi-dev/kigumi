package sem

import "kigumi/internal/syntax"

// crossOwnedRoot claims the box a slot copy still costs a root this body owns: the store writes through its field, so the root keeps the boxed form.
func (c *checker) crossOwnedRoot(lhs syntax.NodeID) {
	if c.t.View(lhs).Base().Kind() == syntax.Ident {
		return
	}
	ent := c.rootOf(lhs)
	if ent == 0 {
		return
	}
	e := &c.r.Entities[ent]
	if e.Kind != EntLocal && e.Kind != EntParam || e.Flags&(EfCapturedMut|EfAddrTaken) != 0 || c.r.IsMutSelf(ent) || c.r.RefCellParam(ent) {
		return
	}
	c.crossDirect(lhs, c.placeRootType(lhs))
}
