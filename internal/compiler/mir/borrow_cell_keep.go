package mir

import (
	"slices"

	"kigumi/internal/compiler/sem"
)

// primSummary is a bodiless fn's summary by its declared shape: a parameter that reaches shared storage may keep
// every other parameter its shape lets it keep (keepableParam); it returns nothing it did not take.
func (c *borrowCheck) primSummary(e sem.EntityID) calleeSummary {
	sum := calleeSummary{stores: map[sem.EntityID][]uint32{}}
	params := c.declaredParams(e)
	for _, q := range params {
		if q == 0 || !c.sharedStorage(c.p.R.Entity(q).Type) {
			continue
		}
		for _, v := range params {
			if v != q && c.keepableParam(v) {
				sum.stores[q] = append(sum.stores[q], uint32(v))
			}
		}
	}
	return sum
}

// keepableParam reports a parameter whose value a callee may keep: one able to hold a Future, a Task or a callable,
// unless it is taken by shared reference or is a callback declared non-escaping.
func (c *borrowCheck) keepableParam(v sem.EntityID) bool {
	if v == 0 {
		return false
	}
	t := c.p.R.Entity(v).Type
	if c.p.R.FnShapeOf(t) != 0 && c.p.R.Entity(v).Flags&sem.EfEscaping == 0 {
		return false
	}
	return c.keepable(t)
}

// keepable reports a value of t able to hold a Future, a Task or something callable; a borrow keeps nothing.
func (c *borrowCheck) keepable(t sem.TypeID) bool {
	return c.p.R.Types.Kind(t) != sem.KRef && (c.typeRuns(t, runsHandle) || c.typeRuns(t, runsCallable))
}

// methodOf is the method a method value closure runs on the receiver it holds, or 0.
func (c *borrowCheck) methodOf(e sem.EntityID) sem.EntityID {
	if e == 0 || c.p.R.Entity(e).Kind != sem.EntClosure {
		return 0
	}
	return c.p.R.Closure(e).Method
}

// sharedStorage reports whether a value of t, held or borrowed, reaches storage its copies share: a type keeping
// something keepable outside its layout (phantomHandle), a raw pointer to one, or an open type; a plain fn holds none.
func (c *borrowCheck) sharedStorage(t sem.TypeID) bool {
	if c.p.R.Types.Kind(t) == sem.KRef {
		t = c.p.R.Types.Node(t).Elem
	}
	return c.typeRuns(t, runsShared)
}

func (c *borrowCheck) sharedNode(t sem.TypeID) bool {
	switch n := c.p.R.Types.Node(t); n.Kind {
	case sem.KIface, sem.KVar, sem.KClosure:
		return true
	case sem.KPtr:
		return c.keepable(n.Elem)
	case sem.KNamed:
		return c.p.R.Entity(n.Ent).Kind == sem.EntType && c.phantomHandle(t)
	}
	return false
}

// phantomHandle reports a keepable type argument of t that t's declared layout never mentions, so the runtime keeps
// it in storage of its own: an opaque std cell or container, or a resource holding a handle to one.
func (c *borrowCheck) phantomHandle(t sem.TypeID) bool {
	n := c.p.R.Types.Node(t)
	d := c.p.R.TypeDecl(n.Ent)
	if d == nil {
		return false
	}
	var layout []sem.TypeID
	if d.Underlying != 0 {
		layout = append(layout, d.Underlying)
	}
	for _, f := range d.Fields {
		layout = append(layout, c.p.R.Entity(f).Type)
	}
	for _, v := range d.Variants {
		layout = append(layout, c.p.R.Variant(v).Payload...)
	}
	for i, p := range d.Params {
		mentions := func(lt sem.TypeID) bool {
			return c.typeHolds(lt, func(x sem.TypeID) bool {
				return c.p.R.Types.Kind(x) == sem.KParam && c.p.R.Types.Node(x).Ent == p
			})
		}
		if i < len(n.Args) && c.keepable(n.Args[i]) && !slices.ContainsFunc(layout, mentions) {
			return true
		}
	}
	return false
}
