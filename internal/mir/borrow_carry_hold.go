package mir

import "kigumi/internal/sem"

// mayCarry reports whether a result of type ret can keep an argument of type arg inside it.
func (c *borrowCheck) mayCarry(ret, arg sem.TypeID) bool {
	return c.p.R.Types.Contains(ret, arg) || c.holdsDeferred(arg) && c.holdsDeferred(ret)
}

// holdsDeferred reports whether a value of type t can keep a Future, a Task or a capture
// inside it: through a type argument, an element, or a field or payload of a named type.
func (c *borrowCheck) holdsDeferred(t sem.TypeID) bool {
	seen := map[sem.EntityID]bool{}
	var walk func(sem.TypeID) bool
	walk = func(t sem.TypeID) bool {
		if t == 0 {
			return false
		}
		n := c.p.R.Types.Node(t)
		switch {
		case n.Kind == sem.KIface || n.Kind == sem.KClosure:
			return true
		case n.Kind == sem.KNamed && (n.Ent == c.p.R.Types.FutureEnt() || c.p.R.IsTaskType(t)):
			return true
		}
		if walk(n.Elem) {
			return true
		}
		for _, a := range n.Args {
			if walk(a) {
				return true
			}
		}
		if n.Kind != sem.KNamed || seen[n.Ent] || c.p.R.Entity(n.Ent).Kind != sem.EntType {
			return false
		}
		seen[n.Ent] = true
		d := c.p.R.TypeDecl(n.Ent)
		if d == nil {
			return false
		}
		if walk(d.Underlying) {
			return true
		}
		for _, f := range d.Fields {
			if walk(c.p.R.Entity(f).Type) {
				return true
			}
		}
		for _, v := range d.Variants {
			for _, pt := range c.p.R.Variant(v).Payload {
				if walk(pt) {
					return true
				}
			}
		}
		return false
	}
	return walk(t)
}
