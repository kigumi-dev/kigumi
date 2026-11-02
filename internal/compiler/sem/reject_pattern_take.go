package sem

import "kigumi/internal/compiler/syntax"

// rejectScrutineeTake: a pattern binding part of its scrutinee by value takes it out of each value
// leaf the way a field read does.
func (c *checker) rejectScrutineeTake(expr syntax.NodeID, bound []EntityID) {
	if !c.bindsOwned(bound) {
		return
	}
	c.rejectOperandTake(expr, 0, takeBound)
}

func (c *checker) bindsOwned(ents []EntityID) bool {
	for _, e := range ents {
		if c.ownsType(c.r.Entities[e].Type) {
			return true
		}
	}
	return false
}

// rejectDropDestructure: a value whose type defines a `drop` cannot be taken apart by a pattern that
// moves a part out, since the `drop` still runs on the whole; only that drop's own `self` can.
func (c *checker) rejectDropDestructure(p, expr syntax.NodeID) bool {
	if k := c.t.Kind(p); (k == syntax.PatRecord || k == syntax.PatCtor) && !c.dropSelf(expr) {
		if t := c.vars.resolve(c.info.Types[p]); c.customDrop(t) {
			if name := c.movedBinding(p); name != "" {
				c.errAt(p, cMoveDropType, name, t, name)
				return true
			}
		}
	}
	for ch := range c.t.View(p).Children() {
		if c.rejectDropDestructure(ch.ID(), 0) {
			return true
		}
	}
	return false
}

// movedBinding names a binding under p that takes an owned value, or is empty.
func (c *checker) movedBinding(p syntax.NodeID) string {
	if e := c.info.Defs[p]; e != 0 && c.r.Entities[e].Kind == EntLocal && c.ownsType(c.r.Entities[e].Type) {
		return c.r.Entities[e].Name
	}
	for ch := range c.t.View(p).Children() {
		if name := c.movedBinding(ch.ID()); name != "" {
			return name
		}
	}
	return ""
}
