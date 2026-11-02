package borrow

import "kigumi/internal/compiler/sem"

// holdsNoCallable reports whether no value of type t can be or contain a callable, from its structure alone: a type
// parameter other than a lifetime, an interface or an inference variable might.
func (c *borrowCheck) holdsNoCallable(t sem.TypeID, seen map[sem.TypeID]bool) bool {
	r, tt := c.p.R, c.p.R.Types
	if seen[t] {
		return true
	}
	switch n := tt.Node(t); n.Kind {
	case sem.KPrim, sem.KUntyped, sem.KConst:
		return true
	case sem.KParam:
		return r.TypeParam(n.Ent).IsLifetime
	case sem.KRef, sem.KPtr:
		return c.holdsNoCallable(n.Elem, seen)
	case sem.KNamed:
		seen[t] = true
		decl := r.TypeDecl(n.Ent)
		subst := map[sem.EntityID]sem.TypeID{}
		for i, p := range decl.Params {
			if i < len(n.Args) {
				subst[p] = n.Args[i]
			}
		}
		parts := append([]sem.TypeID(nil), n.Args...)
		for _, f := range decl.Fields {
			parts = append(parts, tt.Subst(r.Entity(f).Type, subst))
		}
		for _, v := range decl.Variants {
			for _, pt := range r.Variant(v).Payload {
				parts = append(parts, tt.Subst(pt, subst))
			}
		}
		if decl.Underlying != 0 {
			parts = append(parts, tt.Subst(decl.Underlying, subst))
		}
		for _, p := range parts {
			if !c.holdsNoCallable(p, seen) {
				return false
			}
		}
		return true
	}
	return false
}
