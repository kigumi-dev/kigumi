package borrow

import "kigumi/internal/compiler/sem"

func (c *borrowCheck) typeHolds(t sem.TypeID, hit func(sem.TypeID) bool) bool {
	return c.walkTypes(t, hit, nil, false)
}

func (c *borrowCheck) valueHolds(t sem.TypeID, hit, stop func(sem.TypeID) bool) bool {
	return c.walkTypes(t, hit, stop, true)
}

func (c *borrowCheck) walkTypes(t sem.TypeID, hit, stop func(sem.TypeID) bool, stored bool) bool {
	seen := map[sem.TypeID]bool{}
	var walk func(sem.TypeID) bool
	walk = func(t sem.TypeID) bool {
		if t == 0 || seen[t] {
			return false
		}
		if len(seen) > 512 {
			return true
		}
		seen[t] = true
		if hit(t) {
			return true
		}
		if stop != nil && stop(t) {
			return false
		}
		n := c.p.R.Types.Node(t)
		if walk(n.Elem) {
			return true
		}
		var d *sem.TypeDeclInfo
		if n.Kind == sem.KNamed && c.p.R.Entity(n.Ent).Kind == sem.EntType {
			d = c.p.R.TypeDecl(n.Ent)
		}
		if !stored || d == nil || d.Form == sem.FormOpaque {
			for _, a := range n.Args {
				if walk(a) {
					return true
				}
			}
		}
		if d == nil {
			return false
		}
		sub := map[sem.EntityID]sem.TypeID{}
		for i, p := range d.Params {
			if i < len(n.Args) {
				sub[p] = n.Args[i]
			}
		}
		in := func(t sem.TypeID) bool { return walk(c.p.R.Types.Subst(t, sub)) }
		if in(d.Underlying) {
			return true
		}
		for _, f := range d.Fields {
			if in(c.p.R.Entity(f).Type) {
				return true
			}
		}
		for _, v := range d.Variants {
			for _, pt := range c.p.R.Variant(v).Payload {
				if in(pt) {
					return true
				}
			}
		}
		return false
	}
	return walk(t)
}
