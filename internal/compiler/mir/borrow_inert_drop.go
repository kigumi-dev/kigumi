package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// A holder keeps no count of its reference fields, so a drop that runs no drop code does not use the borrow.
func (c *borrowCheck) inertDrop(l midir.LocalID) bool {
	if _, ok := c.holders[l]; !ok || c.f.Locals[l].Ent == 0 || c.p.R.Types.Kind(c.f.Locals[l].Type) == sem.KRef {
		return false
	}
	return c.inertType(c.f.Locals[l].Type, map[sem.EntityID]bool{})
}

func (c *borrowCheck) inertType(t sem.TypeID, seen map[sem.EntityID]bool) bool {
	tt := c.p.R.Types
	n := tt.Node(t)
	switch n.Kind {
	case sem.KRef:
		return true
	case sem.KParam:
		return c.p.R.TypeParam(n.Ent).IsLifetime
	case sem.KIface, sem.KClosure, sem.KFn, sem.KVar:
		return false
	}
	if c.p.R.IsTaskType(t) {
		return false
	}
	for _, a := range append([]sem.TypeID{n.Elem}, n.Args...) {
		if a != 0 && !c.inertType(a, seen) {
			return false
		}
	}
	if n.Kind != sem.KNamed || seen[n.Ent] || c.p.R.Entity(n.Ent).Kind != sem.EntType {
		return true
	}
	seen[n.Ent] = true
	d := c.p.R.TypeDecl(n.Ent)
	if d == nil {
		return true
	}
	// An opaque type such as Future keeps borrows its type does not show.
	if d.Drop != 0 || d.Form == sem.FormOpaque && n.Ent != tt.ArrayEnt() || d.Underlying != 0 && !c.inertType(d.Underlying, seen) {
		return false
	}
	for _, f := range d.Fields {
		if !c.inertType(c.p.R.Entity(f).Type, seen) {
			return false
		}
	}
	for _, v := range d.Variants {
		for _, pt := range c.p.R.Variant(v).Payload {
			if !c.inertType(pt, seen) {
				return false
			}
		}
	}
	return true
}
