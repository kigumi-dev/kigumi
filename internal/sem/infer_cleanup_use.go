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
	if c.conflictUse(a, p, true) {
		return
	}
	if c.cleanup > 0 && !c.declaredIn(p.ent, c.cleanupNode) {
		c.pendingReserve = append(c.pendingReserve, p)
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
	if len(c.reserved) == 0 && len(c.pendingReserve) == 0 {
		return
	}
	p, ok := c.placeUnder(lhs)
	if !ok {
		return
	}
	if i, held := c.reservedWithin(p.ent, p.fields); held {
		c.reservedError(lhs, cCleanupReservedAssign, c.reserved[p.ent][i])
		return
	}
	for _, q := range c.pendingReserve {
		if q.ent == p.ent && isFieldPrefix(p.fields, q.fields) {
			c.reservedError(lhs, cCleanupReservedAssign, reservation{cleanup: c.cleanupNode, name: c.placeLabel(q)})
			return
		}
	}
}

func (c *checker) reserveCapture(n syntax.NodeID, ent EntityID) {
	if _, held := c.reservedOver(ent, nil); held {
		c.conflictUse(n, reservedPlace{ent: ent}, false)
	}
}
