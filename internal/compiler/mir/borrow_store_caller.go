package mir

import "kigumi/internal/compiler/sem"

// checkHeldCallers holds a store that lands, through a `&mut` a local holder keeps, in a place of the caller to the
// rule for a store into that place itself: only a value carrying this function's own lifetime may go in.
func (c *borrowCheck) checkHeldCallers(fl storeFlow, in Inst) {
	_, callers := c.storeReach(fl)
	// A closure has no lifetime of its own, so only what it borrows of its own frame is caught here; the rest reaches
	// its caller's flows through its summary.
	closure := c.p.R.Entity(c.f.Ent).Kind != sem.EntFn
	for _, p := range callers {
		place := c.f.Locals[p].Type
		if c.p.R.Types.Kind(place) == sem.KRef {
			place = c.p.R.Types.Node(place).Elem
		}
		if c.declaredLifetime(place) == 0 && !c.namesTypeParam(place) {
			continue
		}
		for _, v := range fl.values {
			b, has := c.storedInfo(fl, v, true)
			if has && closure {
				b = c.frameBorrows(b)
				has = b != nil
			}
			if has && !b.empty && !c.fitsOwnLifetime(b, place) {
				c.reportCode(in.Node, v.arg, "borrow-store-outlives", c.f.Locals[p].Name)
				return
			}
		}
	}
}
