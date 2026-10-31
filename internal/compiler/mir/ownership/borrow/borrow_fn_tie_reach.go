package borrow

import (
	"slices"

	"kigumi/internal/compiler/sem"
)

// reaches asks whether the callee can build p from arg (not whether arg contains p); a receiver's type parameters may be the method's copies.
func (c *borrowCheck) reaches(arg sem.TypeID, recv bool, p sem.TypeID) bool {
	if !c.mayHoldBorrow(p) {
		return false
	}
	if arg == 0 || c.contains(arg, 0, map[sem.TypeID]bool{}) {
		return true
	}
	if ps := c.typeParamsOf(p); len(ps) > 0 {
		own := c.typeParamsOf(arg)
		if sharesParam(own, ps) || recv && len(own) > 0 || slices.ContainsFunc(ps, c.constrained) {
			return true
		}
	}
	var leaves []sem.TypeID
	c.borrowLeaves(p, map[sem.TypeID]bool{}, &leaves)
	return slices.ContainsFunc(leaves, func(x sem.TypeID) bool { return x == 0 || c.contains(arg, x, map[sem.TypeID]bool{}) })
}

func (c *borrowCheck) mayHoldBorrow(t sem.TypeID) bool {
	return c.facts().HoldsBorrow(t) || c.facts().NamesTypeParam(t) || c.contains(t, 0, map[sem.TypeID]bool{})
}

// borrowLeaves: a reference's referent, a named value itself and its borrow-holding parts; 0 stands for anything.
func (c *borrowCheck) borrowLeaves(t sem.TypeID, seen map[sem.TypeID]bool, out *[]sem.TypeID) {
	if t == 0 || seen[t] {
		return
	}
	seen[t] = true
	switch c.p.R.Types.Kind(t) {
	case sem.KRef:
		*out = append(*out, c.peelRef(t))
		c.borrowLeaves(c.p.R.Types.Node(t).Elem, seen, out)
	case sem.KFn, sem.KClosure, sem.KIface, sem.KPtr, sem.KVar:
		*out = append(*out, 0)
	case sem.KNamed:
		*out = append(*out, t)
		if !c.facts().HoldsBorrow(t) {
			return
		}
		parts, opaque := c.namedParts(t)
		if opaque {
			*out = append(*out, 0)
		}
		for _, p := range parts {
			c.borrowLeaves(p, seen, out)
		}
	}
}

func (c *borrowCheck) holdsMutRef(t sem.TypeID) bool {
	return c.typeHolds(t, func(t sem.TypeID) bool {
		n := c.p.R.Types.Node(t)
		switch n.Kind {
		case sem.KRef:
			return c.p.R.Types.RefMut(t)
		case sem.KParam:
			return !c.p.R.TypeParam(n.Ent).IsLifetime
		case sem.KNamed:
			_, opaque := c.namedParts(t)
			return opaque && c.facts().DeclaredLifetime(t) != 0
		}
		return n.Kind == sem.KFn || n.Kind == sem.KClosure || n.Kind == sem.KIface || n.Kind == sem.KPtr || n.Kind == sem.KVar
	})
}

func (c *borrowCheck) constrained(p sem.EntityID) bool {
	return len(c.p.R.TypeParam(p).Constraints) > 0
}

func (c *borrowCheck) peelRef(t sem.TypeID) sem.TypeID {
	for c.p.R.Types.Kind(t) == sem.KRef {
		t = c.p.R.Types.Node(t).Elem
	}
	return t
}

// contains treats a function, an interface, a pointer or a constrained type parameter as able to make any value; x 0 asks only for those.
func (c *borrowCheck) contains(t, x sem.TypeID, seen map[sem.TypeID]bool) bool {
	t = c.peelRef(t)
	if t == 0 || seen[t] {
		return false
	}
	seen[t] = true
	if x != 0 && c.sameShape(t, x) {
		return true
	}
	n := c.p.R.Types.Node(t)
	switch n.Kind {
	case sem.KFn, sem.KClosure, sem.KIface, sem.KPtr, sem.KVar, sem.KPoison, sem.KUntyped:
		return true
	case sem.KParam:
		return !c.p.R.TypeParam(n.Ent).IsLifetime && c.constrained(n.Ent)
	case sem.KPrim:
		return x != 0 && stringLike(t) && stringLike(x)
	case sem.KNamed:
	default:
		return false
	}
	parts, opaque := c.namedParts(t)
	return x != 0 && opaque || slices.ContainsFunc(parts, func(p sem.TypeID) bool { return c.contains(p, x, seen) })
}

func (c *borrowCheck) namedParts(t sem.TypeID) (parts []sem.TypeID, opaque bool) {
	n := c.p.R.Types.Node(t)
	parts = append([]sem.TypeID{n.Elem}, n.Args...)
	opaque = len(n.Args) == 0
	if d := c.namedDecl(n.Ent, len(n.Args)); d != nil {
		opaque = opaque && len(d.Fields) == 0 && len(d.Variants) == 0 && d.Underlying == 0
		parts = append(append(parts, d.Underlying), c.p.R.AggFields(t)...)
		for _, v := range d.Variants {
			parts = append(parts, c.p.R.AggPayload(t, v)...)
		}
	}
	return parts, opaque
}

func (c *borrowCheck) namedDecl(ent sem.EntityID, args int) *sem.TypeDeclInfo {
	if c.p.R.Entity(ent).Kind != sem.EntType {
		return nil
	}
	if d := c.p.R.TypeDecl(ent); len(d.Params) == args {
		return d
	}
	return nil
}

func stringLike(t sem.TypeID) bool {
	return t == sem.TyString || t == sem.TyBytes
}

func (c *borrowCheck) sameShape(a, b sem.TypeID) bool {
	a, b = c.peelRef(a), c.peelRef(b)
	if a == b {
		return true
	}
	na, nb := c.p.R.Types.Node(a), c.p.R.Types.Node(b)
	if c.isLifetime(na.Kind, na.Ent) || c.isLifetime(nb.Kind, nb.Ent) {
		return true
	}
	if na.Kind != nb.Kind || na.Kind == sem.KPrim || na.Ent != nb.Ent || len(na.Args) != len(nb.Args) {
		return false
	}
	if (na.Elem != 0 || nb.Elem != 0) && !c.sameShape(na.Elem, nb.Elem) {
		return false
	}
	for i := range na.Args {
		if !c.sameShape(na.Args[i], nb.Args[i]) {
			return false
		}
	}
	return true
}

func (c *borrowCheck) isLifetime(k sem.TypeKind, ent sem.EntityID) bool {
	return k == sem.KParam && c.p.R.TypeParam(ent).IsLifetime
}
