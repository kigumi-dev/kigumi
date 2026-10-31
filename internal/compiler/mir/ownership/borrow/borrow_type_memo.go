package borrow

import (
	"math"
	"slices"

	"kigumi/internal/compiler/sem"
)

// borrowTypeMemo caches answers about types, which no longer change once sem is done.
type borrowTypeMemo struct {
	lifetimes map[sem.TypeID]sem.EntityID
	params    map[sem.TypeID]bool
	holds     map[sem.TypeID]bool
	// decls is declYes or declNo for a settled declaration; scc holds those whose component is still open.
	decls      map[sem.EntityID]int
	scc        sccStack[sem.EntityID]
	expansions int
}

const (
	declNo  = -1
	declYes = -2
	noCut   = math.MaxInt
)

// DeclaredLifetime is the first named lifetime t's own generic arguments carry, 0 for none.
func (c *TypeFacts) DeclaredLifetime(t sem.TypeID) sem.EntityID {
	if t == 0 {
		return 0
	}
	m := &c.memo
	if lt, ok := m.lifetimes[t]; ok {
		return lt
	}
	n := c.r.Types.Node(t)
	var lt sem.EntityID
	if n.Kind == sem.KParam {
		if c.r.TypeParam(n.Ent).IsLifetime {
			lt = n.Ent
		}
	} else if lt = c.DeclaredLifetime(n.Elem); lt == 0 {
		for _, a := range n.Args {
			if lt = c.DeclaredLifetime(a); lt != 0 {
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

// NamesTypeParam reports whether t names a type parameter, whose instantiation may hold a reference.
func (c *TypeFacts) NamesTypeParam(t sem.TypeID) bool {
	if t == 0 {
		return false
	}
	m := &c.memo
	if named, ok := m.params[t]; ok {
		return named
	}
	n := c.r.Types.Node(t)
	named := n.Kind == sem.KParam && !c.r.TypeParam(n.Ent).IsLifetime
	if n.Kind != sem.KParam {
		named = c.NamesTypeParam(n.Elem) || slices.ContainsFunc(n.Args, c.NamesTypeParam)
	}
	if m.params == nil {
		m.params = map[sem.TypeID]bool{}
	}
	m.params[t] = named
	return named
}

// HoldsBorrow reports whether t or a type it can keep inside (a type argument, an element,
// or a field or payload of a named type) is a reference or names a lifetime.
func (c *TypeFacts) HoldsBorrow(t sem.TypeID) bool {
	held, _ := c.holdsBorrowLow(t)
	return held
}

// holdsBorrowLow also returns the lowest stack slot of an open declaration a false answer
// leaned on, noCut for none; that false is cached only once the declaration settles.
func (c *TypeFacts) holdsBorrowLow(t sem.TypeID) (bool, int) {
	if t == 0 {
		return false, noCut
	}
	m := &c.memo
	if held, ok := m.holds[t]; ok {
		return held, noCut
	}
	n := c.r.Types.Node(t)
	held, low := n.Kind == sem.KRef || c.DeclaredLifetime(t) != 0, noCut
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
	if !held && n.Kind == sem.KNamed && c.r.Entity(n.Ent).Kind == sem.EntType {
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
func (c *TypeFacts) declHoldsBorrow(e sem.EntityID) (bool, int) {
	m := &c.memo
	if v, ok := m.decls[e]; ok {
		return v == declYes, noCut
	}
	if slot, open := m.scc.slot(e); open {
		return false, slot
	}
	d := c.r.TypeDecl(e)
	if d == nil {
		return false, noCut
	}
	if m.decls == nil {
		m.decls = map[sem.EntityID]int{}
	}
	m.expansions++
	slot := m.scc.push(e)
	parts := []sem.TypeID{d.Underlying}
	for _, f := range d.Fields {
		parts = append(parts, c.r.Entity(f).Type)
	}
	for _, v := range d.Variants {
		parts = append(parts, c.r.Variant(v).Payload...)
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
	for _, e := range m.scc.pop(slot) {
		m.decls[e] = answer
	}
}
