package mir

import "kigumi/internal/sem"

// mutRefInside reports a `&mut` anywhere in t, a function's signature included.
func (c *borrowCheck) mutRefInside(t sem.TypeID) bool {
	return c.typeHolds(t, func(t sem.TypeID) bool { _, mut := c.p.R.MutRefElem(t); return mut })
}

// mayStoreThrough reports a value of t that may let code store into a place: a `&mut`, a callable, or a value of a
// type parameter, which may be either.
func (c *borrowCheck) mayStoreThrough(t sem.TypeID) bool {
	if v, ok := c.storeThrough[t]; ok {
		return v
	}
	tt := c.p.R.Types
	v := c.typeHolds(t, func(u sem.TypeID) bool {
		switch n := tt.Node(u); n.Kind {
		case sem.KRef:
			return tt.RefMut(u)
		case sem.KFn, sem.KClosure, sem.KIface:
			return true
		case sem.KParam:
			return !c.p.R.TypeParam(n.Ent).IsLifetime
		}
		return false
	})
	if c.storeThrough == nil {
		c.storeThrough = map[sem.TypeID]bool{}
	}
	c.storeThrough[t] = v
	return v
}

// storesMutRef reports a `&mut` a value of t keeps in itself or an element, field or payload, which a callee can
// store through; a function type keeps none of the references its signature names.
func (c *borrowCheck) storesMutRef(t sem.TypeID) bool {
	tt := c.p.R.Types
	return c.typeHoldsUntil(t, func(t sem.TypeID) bool { return tt.Kind(t) == sem.KRef && tt.RefMut(t) }, func(t sem.TypeID) bool {
		k := tt.Kind(t)
		return k == sem.KFn || k == sem.KClosure || k == sem.KIface
	})
}

// plainData reports whether a value of t holds no `&mut`, closure, Future or type parameter, so it can keep a
// borrow only as a shared one.
func (c *borrowCheck) plainData(t sem.TypeID) bool {
	plain, ok := c.plainTypes[t]
	if !ok {
		plain = !c.namesTypeParam(t) && !c.holdsDeferred(t) && !c.mutRefInside(t)
		if c.plainTypes == nil {
			c.plainTypes = map[sem.TypeID]bool{}
		}
		c.plainTypes[t] = plain
	}
	return plain
}
