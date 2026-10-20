package sem

import (
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

func (c *checker) rejectLoopConsume(n syntax.NodeID, consumed []reservedPlace) {
	if c.t.Kind(n) != syntax.DeferStmt || c.cleanup > 0 {
		return
	}
	at := c.t.Span(n)
	for _, loop := range c.loopSpans {
		if loop.Start > at.Start || at.End > loop.End || c.lambdaWithin(loop) {
			continue
		}
		for _, p := range consumed {
			if e := &c.r.Entities[p.ent]; e.File == c.f && c.t.Span(e.Node).Start >= loop.Start && c.t.Span(e.Node).End <= loop.End {
				continue
			}
			name := c.placeLabel(p)
			if d, ok := c.r.diagAt(c.f, n, cCleanupReadsMoved, name); ok {
				c.r.emitDiag(c.f, d.WithNote(c.loc(p.node), "`"+name+"` is moved here by the previous run of this cleanup"))
			}
			return
		}
	}
}

func (c *checker) lambdaWithin(loop token.Span) bool {
	for _, ctx := range c.lambdas {
		if c.t.Span(c.r.Entities[ctx.ent].Node).Start >= loop.Start {
			return true
		}
	}
	return false
}
