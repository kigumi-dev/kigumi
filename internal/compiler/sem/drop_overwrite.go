package sem

import "kigumi/internal/compiler/syntax"

// overwriteDrop charges the drop of the value a plain assignment to lhs replaces, less what an unconditional move already took out of it.
func (c *checker) overwriteDrop(lhs syntax.NodeID, target TypeID) {
	target = c.vars.resolve(target)
	trusted := c.loop == 0 && c.cleanup == 0
	if ent, ok := c.identPlaceEnt(lhs); ok {
		if whole, _ := c.consumed.moved(ent); !trusted || !whole {
			c.dropRemaining(target, nil, lhs)
		}
		return
	}
	var moved [][]EntityID
	if p, ok := c.placeUnder(lhs); ok && trusted {
		whole, parts := c.consumed.moved(p.ent)
		if whole {
			return
		}
		for _, m := range parts {
			switch {
			case isFieldPrefix(m, p.fields):
				return
			case isFieldPrefix(p.fields, m):
				moved = append(moved, m[len(p.fields):])
			}
		}
	}
	c.dropRemaining(target, moved, lhs)
}
