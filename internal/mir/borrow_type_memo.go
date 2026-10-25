package mir

import (
	"math"
	"slices"

	"kigumi/internal/sem"
)

// borrowTypeMemo caches answers about types, which no longer change once sem is done.
type borrowTypeMemo struct {
	lifetimes map[sem.TypeID]sem.EntityID
	params    map[sem.TypeID]bool
	holds     map[sem.TypeID]bool
	// decls is declYes, declNo, or the 1-based slot in stack of a declaration whose component is still open.
	decls      map[sem.EntityID]int
	stack      []sem.EntityID
	expansions int
}

const (
	declNo  = -1
	declYes = -2
	noCut   = math.MaxInt
)

// declaredLifetime is the first named lifetime t's own generic arguments carry, 0 for none.
func (c *borrowCheck) declaredLifetime(t sem.TypeID) sem.EntityID {
	if t == 0 {
		return 0
	}
	m := &c.typeMemo
	if lt, ok := m.lifetimes[t]; ok {
		return lt
	}
	n := c.p.R.Types.Node(t)
	var lt sem.EntityID
	if n.Kind == sem.KParam {
		if c.p.R.TypeParam(n.Ent).IsLifetime {
			lt = n.Ent
		}
	} else if lt = c.declaredLifetime(n.Elem); lt == 0 {
		for _, a := range n.Args {
			if lt = c.declaredLifetime(a); lt != 0 {
				break
			}
		}
	}
	if m.lifetimes == nil {
		m.lifetimes = map[sem.TypeID]sem.EntityID{}
	}
	m.lifetimes[t] = lt
	return lt
}

// namesTypeParam reports whether t names a type parameter, whose instantiation may hold a reference.
func (c *borrowCheck) namesTypeParam(t sem.TypeID) bool {
	if t == 0 {
		return false
	}
	m := &c.typeMemo
	if named, ok := m.params[t]; ok {
		return named
	}
	n := c.p.R.Types.Node(t)
	named := n.Kind == sem.KParam && !c.p.R.TypeParam(n.Ent).IsLifetime
	if n.Kind != sem.KParam {
		named = c.namesTypeParam(n.Elem) || slices.ContainsFunc(n.Args, c.namesTypeParam)
	}
	if m.params == nil {
		m.params = map[sem.TypeID]bool{}
	}
	m.params[t] = named
	return named
}

// holdsBorrow reports whether t or a type it can keep inside (a type argument, an element,
// or a field or payload of a named type) is a reference or names a lifetime.
func (c *borrowCheck) holdsBorrow(t sem.TypeID) bool {
	held, _ := c.holdsBorrowLow(t)
	return held
}

// holdsBorrowLow also returns the lowest stack slot of an open declaration a false answer
// leaned on, noCut for none; that false is cached only once the declaration settles.
func (c *borrowCheck) holdsBorrowLow(t sem.TypeID) (bool, int) {
	if t == 0 {
		return false, noCut
	}
	m := &c.typeMemo
	if held, ok := m.holds[t]; ok {
		return held, noCut
	}
	n := c.p.R.Types.Node(t)
	held, low := n.Kind == sem.KRef || c.declaredLifetime(t) != 0, noCut
	visit := func(u sem.TypeID) {
		if !held {
			var l int
			held, l = c.holdsBorrowLow(u)
			low = min(low, l)
		}
	}
	visit(n.Elem)
	for _, a := range n.Args {
		visit(a)
	}
	if !held && n.Kind == sem.KNamed && c.p.R.Entity(n.Ent).Kind == sem.EntType {
		var l int
		held, l = c.declHoldsBorrow(n.Ent)
		low = min(low, l)
	}
	if !held && low != noCut {
		return false, low
	}
	if m.holds == nil {
		m.holds = map[sem.TypeID]bool{}
	}
	m.holds[t] = held
	return held, noCut
}

// declHoldsBorrow is holdsBorrowLow over the underlying type, fields and payloads of the named type e;
// mutually recursive declarations settle together, as Tarjan's strongly connected components.
func (c *borrowCheck) declHoldsBorrow(e sem.EntityID) (bool, int) {
	m := &c.typeMemo
	switch v, ok := m.decls[e]; {
	case !ok:
	case v == declYes:
		return true, noCut
	case v == declNo:
		return false, noCut
	default:
		return false, v
	}
	d := c.p.R.TypeDecl(e)
	if d == nil {
		return false, noCut
	}
	if m.decls == nil {
		m.decls = map[sem.EntityID]int{}
	}
	m.expansions++
	m.stack = append(m.stack, e)
	slot := len(m.stack)
	m.decls[e] = slot
	parts := []sem.TypeID{d.Underlying}
	for _, f := range d.Fields {
		parts = append(parts, c.p.R.Entity(f).Type)
	}
	for _, v := range d.Variants {
		parts = append(parts, c.p.R.Variant(v).Payload...)
	}
	held, low := false, noCut
	for _, p := range parts {
		if held {
			break
		}
		var l int
		held, l = c.holdsBorrowLow(p)
		low = min(low, l)
	}
	switch {
	case held:
		m.settle(slot, declYes)
	case low >= slot:
		m.settle(slot, declNo)
	default:
		return false, low
	}
	return held, noCut
}

// settle gives answer to the declaration at slot and to the open ones above it, all in its component.
func (m *borrowTypeMemo) settle(slot, answer int) {
	for _, e := range m.stack[slot-1:] {
		m.decls[e] = answer
	}
	m.stack = m.stack[:slot-1]
}
