package sem

import "kigumi/internal/syntax"

// partAssign is a value stored into part of a local, through a field or index place or a `mut self` call.
type partAssign struct {
	field EntityID
	value syntax.NodeID
}

func (c *checker) notePartAssign(place, value syntax.NodeID) {
	root, field := c.placeRoot(place)
	if root == 0 || !c.holdsLentRef(c.info.Types[c.t.Unparen(value)]) {
		return
	}
	if c.held.partAssigns == nil {
		c.held.partAssigns = map[EntityID][]partAssign{}
	}
	c.held.partAssigns[root] = append(c.held.partAssigns[root], partAssign{field, value})
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
