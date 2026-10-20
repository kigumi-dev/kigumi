package sem

import "kigumi/internal/syntax"

func (c *checker) reserveUse(a syntax.NodeID) {
	p, ok := c.placeUnder(a)
	if !ok || !c.ownsValue(p.ent) {
		return
	}
	if len(p.fields) > 0 && !c.ownsType(c.info.Types[c.stripParen(a)]) {
		return
	}
	c.reservePlace(a, p)
}

func (c *checker) reservePlace(a syntax.NodeID, p reservedPlace) {
	if c.conflictUse(a, p, true) {
		return
	}
	if c.cleanup > 0 && !c.declaredIn(p.ent, c.cleanupNode) {
		c.consumeAfterAssign(a, p)
		c.cleanupLog.consumed = append(c.cleanupLog.consumed, p)
	}
}

// consumeAfterAssign flags an earlier cleanup that assigns what this one consumes: cleanups run in reverse, so the assignment would revive a value nothing drops.
func (c *checker) consumeAfterAssign(a syntax.NodeID, p reservedPlace) {
	for _, s := range c.cleanupLog.earlier {
		if s.place.ent == p.ent && isFieldPrefix(s.place.fields, p.fields) {
			c.reservedError(s.node, cCleanupReservedAssign, reservation{cleanup: c.cleanupNode, name: c.placeLabel(p)})
		}
	}
}

func (c *checker) conflictUse(a syntax.NodeID, p reservedPlace, exitOK bool) bool {
	i, held := c.reservedOver(p.ent, p.fields)
	if !held {
		return false
	}
	r := c.reserved[p.ent][i]
	if exitOK && c.exitMoveCandidate(a, r) {
		return true
	}
	code := cCleanupReservedMove
	if c.cleanup > 0 {
		code = cCleanupReservedTwice
		c.reserved[p.ent][i].fail = false
	}
	c.reservedError(a, code, r)
	return true
}

func (c *checker) reserveAssign(lhs syntax.NodeID) {
	if len(c.reserved) == 0 && len(c.cleanupLog.consumed) == 0 && c.cleanup == 0 {
		return
	}
	p, ok := c.placeUnder(lhs)
	if !ok {
		return
	}
	if c.cleanup > 0 && c.ownsValue(p.ent) && !c.declaredIn(p.ent, c.cleanupNode) {
		c.cleanupLog.assigned = append(c.cleanupLog.assigned, assignSite{place: p, node: lhs})
	}
	if i, held := c.reservedWithin(p.ent, p.fields); held {
		c.reservedError(lhs, cCleanupReservedAssign, c.reserved[p.ent][i])
		return
	}
	for _, q := range c.cleanupLog.consumed {
		if q.ent == p.ent && isFieldPrefix(p.fields, q.fields) {
			c.reservedError(lhs, cCleanupReservedAssign, reservation{cleanup: c.cleanupNode, name: c.placeLabel(q)})
			return
		}
	}
}
