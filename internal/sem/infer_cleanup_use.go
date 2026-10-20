package sem

import "kigumi/internal/syntax"

func (c *checker) consumedPlace(a syntax.NodeID) (reservedPlace, bool) {
	p, ok := c.placeUnder(a)
	if !ok || !c.ownsValue(p.ent) {
		return p, false
	}
	return p, len(p.fields) == 0 || c.ownsType(c.info.Types[c.stripParen(a)])
}

func (c *checker) reserveUse(a syntax.NodeID) {
	if p, ok := c.consumedPlace(a); ok {
		c.reservePlace(a, p)
	}
}

func (c *checker) reservePlace(a syntax.NodeID, p reservedPlace) {
	c.noteLambdaConsume(a, p)
	if c.conflictUse(a, p, true) {
		return
	}
	if c.cleanup > 0 && !c.declaredIn(p.ent, c.cleanupNode) {
		c.consumeAfterAssign(a, p)
		p.node = a
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

func (c *checker) reserveAssign(lhs syntax.NodeID, target TypeID) {
	p, ok := c.placeUnder(lhs)
	if ok && c.ownsValue(p.ent) {
		c.held.plainAssigns = append(c.held.plainAssigns, assignSite{place: p, node: lhs})
	}
	if len(c.reserved) == 0 && len(c.cleanupLog.consumed) == 0 && c.cleanup == 0 {
		return
	}
	if !ok {
		c.reserveIndexAssign(lhs, target)
		return
	}
	if c.cleanup > 0 && c.ownsValue(p.ent) && !c.declaredIn(p.ent, c.cleanupNode) {
		c.cleanupLog.assigned = append(c.cleanupLog.assigned, assignSite{place: p, node: lhs})
	}
	holds := c.r.mayHoldResource(c.vars.resolve(target), map[TypeID]bool{})
	match := c.reservedWithin
	if holds {
		match = c.reservedOver
	}
	if i, held := match(p.ent, p.fields); held {
		c.reservedError(lhs, cCleanupReservedAssign, c.reserved[p.ent][i])
		return
	}
	for _, q := range c.cleanupLog.consumed {
		if q.ent == p.ent && (isFieldPrefix(p.fields, q.fields) || holds && isFieldPrefix(q.fields, p.fields)) {
			c.reservedError(lhs, cCleanupReservedAssign, reservation{cleanup: c.cleanupNode, name: c.placeLabel(q)})
			return
		}
	}
}

func (c *checker) reserveIndexAssign(lhs syntax.NodeID, target TypeID) {
	owner, ok := c.indexOwner(lhs)
	if !ok || owner.Root == 0 || owner.Deref || c.r.isCopy(c.vars.resolve(target)) {
		return
	}
	if i, held := c.reservedOver(owner.Root, owner.Fields); held {
		c.reservedError(lhs, cCleanupReservedAssign, c.reserved[owner.Root][i])
	}
}

func (c *checker) indexOwner(n syntax.NodeID) (Place, bool) {
	n = c.stripParen(n)
	switch c.t.Kind(n) {
	case syntax.MemberExpr:
		return c.indexOwner(c.t.View(n).AsMember().Base().ID())
	case syntax.BracketExpr:
		base := c.t.View(n).AsBracket().Base().ID()
		if p, ok := c.indexOwner(base); ok {
			return p, true
		}
		return c.buildPlace(c.stripParen(base))
	}
	return Place{}, false
}
