package sem

import (
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

func (c *checker) rejectLoopConsume(n syntax.NodeID, consumed []reservedPlace) {
	if k := c.t.Kind(n); (k != syntax.DeferStmt && k != syntax.Lambda) || c.cleanup > 0 {
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
			if c.assignedBefore(loop, n, p) {
				continue
			}
			name := c.placeLabel(p)
			if d, ok := c.r.diagAt(c.f, n, cCleanupReadsMoved, name); ok {
				c.r.emitDiag(c.f, d.WithNote(c.loc(p.node), c.previousRunNote(n, name)))
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

func (c *checker) assignedBefore(loop token.Span, cleanup syntax.NodeID, p reservedPlace) bool {
	at := c.t.Span(cleanup)
	for _, a := range c.held.plainAssigns {
		sp := c.t.Span(a.node)
		if a.place.ent != p.ent || !isFieldPrefix(a.place.fields, p.fields) || sp.Start < loop.Start || sp.End > at.Start {
			continue
		}
		path := c.pathTo(c.t.Root, a.node)
		for i := len(path) - 2; i >= 0 && c.t.Kind(path[i]) == syntax.Block; {
			if blk := c.t.Span(path[i]); blk.Start < loop.Start {
				break
			} else if blk.End >= at.End {
				return true
			}
			i = c.bareBlockParent(path, i)
		}
	}
	return false
}

func (c *checker) bareBlockParent(path []syntax.NodeID, i int) int {
	if i >= 1 && c.t.Kind(path[i-1]) == syntax.Block {
		return i - 1
	}
	if i >= 2 && c.t.Kind(path[i-1]) == syntax.ExprStmt && c.t.Kind(path[i-2]) == syntax.Block {
		return i - 2
	}
	return -1
}

func (c *checker) previousRunNote(n syntax.NodeID, name string) string {
	if c.t.Kind(n) == syntax.Lambda {
		return "`" + name + "` is consumed here by the closure made in the previous iteration"
	}
	return "`" + name + "` is moved here by the previous run of this cleanup"
}
