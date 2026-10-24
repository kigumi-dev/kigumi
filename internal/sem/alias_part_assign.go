package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

// partAssign is a value stored into part of a local, through a field or index place, a `mut self` call or a `&mut` argument.
type partAssign struct {
	field EntityID
	value syntax.NodeID
}

// storeReach: a store lands in its starts, also behind a `&mut` their types can hold, or behind anything they hold.
type storeReach int

const (
	reachHere storeReach = iota
	reachHeld
	reachBehind
)

func (c *checker) notePartAssign(place, value syntax.NodeID) {
	reach := reachHere
	if c.crossesRef(place, false) {
		reach = reachBehind
	}
	c.recordStore(place, value, c.info.Types[c.t.Unparen(place)], reach)
}

func (c *checker) noteStoreThrough(place, value syntax.NodeID) {
	target := c.pointee(c.info.Types[c.t.Unparen(place)])
	reach := reachHere
	if c.crossesRef(place, true) || c.holdsLentHolder(target) {
		reach = reachBehind
	}
	c.recordStore(place, value, target, reach)
}

func (c *checker) recordStore(place, value syntax.NodeID, target TypeID, reach storeReach) {
	if root, field := c.placeRoot(place); root != 0 {
		c.recordStoreAt([]lentRef{{root, field}}, reach, target, value)
	}
}

func (c *checker) recordStoreAt(starts []lentRef, reach storeReach, target TypeID, value syntax.NodeID) {
	if len(starts) == 0 || !c.holdsLentRef(target) || !c.holdsLentRef(c.info.Types[c.t.Unparen(value)]) {
		return
	}
	if c.held.partAssigns == nil {
		c.held.partAssigns = map[EntityID][]partAssign{}
	}
	for _, to := range c.storeTargets(starts, reach) {
		c.held.partAssigns[to.root] = append(c.held.partAssigns[to.root], partAssign{to.field, value})
	}
}

func (c *checker) pointee(t TypeID) TypeID {
	t = c.vars.resolve(t)
	if n := c.r.Types.Node(t); n.Kind == KRef {
		return n.Elem
	}
	return t
}

func (c *checker) crossesRef(place syntax.NodeID, self bool) bool {
	for n, i := c.t.Unparen(place), 0; n != 0 && i < 64; i++ {
		if (self || i > 0) && c.refMutPlace(n) {
			return true
		}
		if k := c.t.Kind(n); k != syntax.MemberExpr && k != syntax.BracketExpr {
			return false
		}
		n = c.t.Unparen(c.t.View(n).Base().ID())
	}
	return false
}

func (c *checker) refMutPlace(n syntax.NodeID) bool {
	if c.r.Types.RefMut(c.vars.resolve(c.info.Types[n])) {
		return true
	}
	fld := c.info.Uses[n]
	return c.t.Kind(n) == syntax.MemberExpr && fld != 0 && c.r.Entities[fld].Kind == EntField && c.r.Types.RefMut(c.r.Entities[fld].Type)
}

func (c *checker) storeTargets(starts []lentRef, reach storeReach) []lentRef {
	out := slices.Clone(starts)
	for i := 0; i < len(out) && len(out) < 16; i++ {
		at, how := out[i], reachHeld
		if i < len(starts) {
			how = reach
		}
		var next []lentRef
		switch {
		case how == reachHere:
		case c.r.Types.RefMut(c.vars.resolve(c.r.Entities[at.root].Type)):
			for _, from := range c.mutAliasTargets(at.root) {
				next = append(next, from.within(at.field))
			}
		case how == reachBehind || c.holdsLentHolder(c.typeAt(at)):
			next = c.aliasSources(at.root, at.field)
		}
		for _, from := range next {
			if from.root == 0 || slices.Contains(out, from) {
				continue
			}
			if k := c.r.Entities[from.root].Kind; k == EntLocal || k == EntParam {
				out = append(out, from)
			}
		}
	}
	return out
}

func (c *checker) typeAt(at lentRef) TypeID {
	if at.field != 0 {
		return c.r.Entities[at.field].Type
	}
	return c.r.Entities[at.root].Type
}

func (c *checker) mutAliasTargets(ent EntityID) []lentRef {
	if from, ok := c.lentFrom[ent]; ok {
		return []lentRef{from}
	}
	if init, ok := c.aliasInit(ent); ok {
		return c.leafSources(init, 0, true, 0)
	}
	return nil
}

func (c *checker) partAssignSources(ent, field EntityID) []lentRef {
	var out []lentRef
	for _, a := range c.held.partAssigns[ent] {
		if (field == 0 || a.field == 0 || a.field == field) && c.rebindReaches(ent, a.value) {
			out = append(out, c.leafSources(a.value, 0, false, 0)...)
		}
	}
	return out
}
