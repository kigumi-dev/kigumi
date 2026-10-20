package sem

import "kigumi/internal/syntax"

type reservation struct {
	cleanup syntax.NodeID
	scope   ScopeID
	fail    bool
	lambdas int
}

func (c *checker) ownsValue(ent EntityID) bool {
	e := &c.r.Entities[ent]
	if e.Kind != EntLocal && e.Kind != EntParam {
		return false
	}
	t := c.vars.resolve(e.Type)
	return t != TyPoison && c.r.Types.Kind(t) != KRef && !c.r.isCopy(t)
}

func (c *checker) declaredIn(ent EntityID, n syntax.NodeID) bool {
	e := &c.r.Entities[ent]
	if e.File != c.f {
		return false
	}
	inner, outer := c.t.Span(e.Node), c.t.Span(n)
	return inner.Start >= outer.Start && inner.End <= outer.End
}

func (c *checker) reservedError(n syntax.NodeID, code Code, ent EntityID) {
	c.reservedErrorAt(n, code, ent, c.reserved[ent])
}

func (c *checker) reservedErrorAt(n syntax.NodeID, code Code, ent EntityID, r reservation) {
	d, ok := c.r.diagAt(c.f, n, code, c.r.Entities[ent].Name)
	if ok {
		c.r.emitDiag(c.f, d.WithNote(c.loc(r.cleanup), "`"+c.r.Entities[ent].Name+"` is reserved by this cleanup"))
	}
}

func (c *checker) reserveUse(a syntax.NodeID) {
	if c.t.Kind(a) != syntax.Ident {
		return
	}
	ent := c.info.Uses[a]
	if ent == 0 || !c.ownsValue(ent) {
		return
	}
	if r, held := c.reserved[ent]; held {
		if c.exitMoveCandidate(a, ent, r) {
			return
		}
		code := cCleanupReservedMove
		if c.cleanup > 0 {
			code = cCleanupReservedTwice
			r.fail = false
			c.reserved[ent] = r
		}
		c.reservedError(a, code, ent)
		return
	}
	if c.cleanup > 0 && !c.declaredIn(ent, c.cleanupNode) {
		c.pendingReserve = append(c.pendingReserve, ent)
	}
}

func (c *checker) reserveAssign(lhs syntax.NodeID) {
	if len(c.reserved) == 0 || c.t.Kind(lhs) != syntax.Ident {
		return
	}
	if ent := c.info.Uses[lhs]; ent != 0 {
		if _, held := c.reserved[ent]; held {
			c.reservedError(lhs, cCleanupReservedAssign, ent)
		}
	}
}

func (c *checker) holdReservations(n syntax.NodeID, ents []EntityID) {
	for _, ent := range ents {
		if _, held := c.reserved[ent]; held {
			continue
		}
		if c.reserved == nil {
			c.reserved = map[EntityID]reservation{}
		}
		c.reserved[ent] = reservation{cleanup: n, scope: c.scope, fail: c.t.Kind(n) == syntax.DeferFailStmt, lambdas: len(c.lambdas)}
	}
}

func (c *checker) releaseReservations(scope ScopeID) {
	for ent, r := range c.reserved {
		if r.scope == scope {
			delete(c.reserved, ent)
		}
	}
}
