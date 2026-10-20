package sem

import "kigumi/internal/syntax"

// holdLambdaConsumes reserves what a closure consumes when called, in the scope that declares each place, as a cleanup would.
func (c *checker) holdLambdaConsumes(n syntax.NodeID, ctx *lambdaCtx) {
	ps := c.onceConsumes(ctx)
	if len(ps) == 0 || c.cleanup > 0 {
		return
	}
	saved := c.scope
	for _, p := range ps {
		c.scope = c.declScope(p.ent)
		c.holdReservations(n, []reservedPlace{p})
		c.scope = saved
	}
	c.rejectLoopConsume(n, ps)
}

func (c *checker) declScope(ent EntityID) ScopeID {
	for s := c.scope; s != 0; s = c.r.Scopes[s].Parent {
		if c.r.Scopes[s].Names[c.r.Entities[ent].Name].Ent == ent {
			return s
		}
	}
	return c.scope
}
