package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// bodylessKeepsNone reports whether call in of bodyless e can keep nothing it is given: a primitive has no keptSummary,
// so its result and every place an argument lends it must be of types that cannot hold a borrow.
func (c *borrowCheck) bodylessKeepsNone(g *midir.Func, in midir.Inst, e sem.EntityID) bool {
	r := c.p.R
	if c.p.ByEnt[e] != nil || r.Entity(e).Kind != sem.EntFn || ownquery.IsAsyncBody(r, e) || c.ifaceRequirement(e) {
		return false
	}
	if info := r.Fn(e); info.Declared&sem.EffUnsafe != 0 || info.Abi != "" {
		return false
	}
	if in.Op.Yields() && (c.mayHoldBorrow(g.Locals[in.Dst].Type) || c.holdsDeferred(g.Locals[in.Dst].Type)) {
		return false
	}
	for _, a := range in.Args {
		t := g.Locals[a].Type
		if elem, mut := r.MutRefElem(t); mut {
			t = elem
		}
		if c.keepsThrough(g, a) && (c.mayHoldBorrow(t) || c.holdsDeferred(t)) {
			return false
		}
	}
	return true
}

// ownPlaceHoldsNothing reports whether parameter l is param itself and its referent cannot hold a borrow, so the place
// it lends cannot keep it.
func (c *borrowCheck) ownPlaceHoldsNothing(g *midir.Func, l midir.LocalID, param sem.EntityID) bool {
	t := g.Locals[l].Type
	if c.p.R.Types.Kind(t) == sem.KRef {
		t = c.p.R.Types.Node(t).Elem
	}
	return g.Locals[l].Ent == param && !c.mayHoldBorrow(t) && !c.holdsDeferred(t)
}
