package sem

import "kigumi/internal/compiler/syntax"

func (c *checker) pushScope(kind ScopeKind, node syntax.NodeID) ScopeID {
	s := c.r.newScope(kind, c.scope, c.f, node)
	c.r.Scopes[s].Fn = c.fn
	c.info.Scopes[node] = s
	c.scope = s
	return s
}

func (c *checker) popScope(saved ScopeID) {
	if len(c.reserved) > 0 || len(c.cleanupLog.earlier) > 0 {
		c.releaseReservations(c.scope)
	}
	c.scope = saved
}

func (c *checker) setType(n syntax.NodeID, t TypeID) TypeID {
	c.info.Types[n] = t
	c.touched = append(c.touched, n)
	return t
}
