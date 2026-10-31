package sem

import "kigumi/internal/compiler/syntax"

type lambdaConsume struct {
	place reservedPlace
	node  syntax.NodeID
}

func (c *checker) noteLambdaConsume(a syntax.NodeID, p reservedPlace) {
	for _, ctx := range c.lambdas {
		if i, ok := ctx.captured[p.ent]; ok && ctx.list[i].Mode == capMove {
			ctx.consumes = append(ctx.consumes, lambdaConsume{p, a})
		}
	}
}

func (c *checker) noteNestedCaptureMoves(n syntax.NodeID, ctx *lambdaCtx) {
	for _, cp := range ctx.list {
		if cp.Mode == capMove && !cp.OwnCell {
			c.noteLambdaConsume(n, reservedPlace{ent: cp.Local})
		}
	}
}

func (c *checker) noteLambdaTail(body syntax.NodeID) {
	for _, leaf := range c.tailLeaves(body, 0) {
		if p, ok := c.consumedPlace(leaf); ok {
			c.noteLambdaConsume(leaf, p)
		}
	}
}

func (c *checker) lambdaOnce(ctx *lambdaCtx) string {
	if ps := c.onceConsumes(ctx); len(ps) > 0 {
		return c.placeLabel(ps[0])
	}
	return ""
}

func (c *checker) onceConsumes(ctx *lambdaCtx) []reservedPlace {
	var ps []reservedPlace
	for _, k := range ctx.consumes {
		if !c.restoredAfter(k) {
			p := k.place
			p.node = k.node
			ps = append(ps, p)
		}
	}
	return ps
}

func (c *checker) reserverNote(r reservation) string {
	return "`" + r.name + "` is reserved by this cleanup"
}

func (c *checker) restoredAfter(k lambdaConsume) bool {
	at := c.t.Span(k.node)
	for _, a := range c.held.plainAssigns {
		sp := c.t.Span(a.node)
		if a.place.ent != k.place.ent || !isFieldPrefix(a.place.fields, k.place.fields) || sp.Start < at.End {
			continue
		}
		path := c.pathTo(c.t.Root, a.node)
		if len(path) < 2 || c.t.Kind(path[len(path)-2]) != syntax.Block {
			continue
		}
		blk := path[len(path)-2]
		if bs := c.t.Span(blk); bs.Start <= at.Start && bs.End >= sp.End && !c.exitsBetween(blk, k.node, path[len(path)-1]) {
			return true
		}
	}
	return false
}

func (c *checker) exitsBetween(blk, from, to syntax.NodeID) bool {
	at := c.t.Span(from)
	for st := range c.t.View(blk).Children() {
		sp := c.t.Span(st.ID())
		if sp.End < at.Start {
			continue
		}
		if c.hasExit(st.ID(), from) {
			return true
		}
		if st.ID() == to {
			break
		}
	}
	return false
}

func (c *checker) hasExit(n, from syntax.NodeID) bool {
	switch c.t.Kind(n) {
	case syntax.ReturnExpr, syntax.TryExpr, syntax.FailExpr, syntax.BreakExpr, syntax.ContinueExpr:
		return true
	case syntax.Lambda:
		return false
	case syntax.ForExpr:
		if !c.within(from, n) {
			for ch := range c.t.View(n).Children() {
				if c.hasLeave(ch.ID()) {
					return true
				}
			}
			return false
		}
	}
	for ch := range c.t.View(n).Children() {
		if c.hasExit(ch.ID(), from) {
			return true
		}
	}
	return false
}

// hasLeave is hasExit inside a loop that does not hold the consume, where break and continue stay in that loop.
func (c *checker) hasLeave(n syntax.NodeID) bool {
	switch c.t.Kind(n) {
	case syntax.ReturnExpr, syntax.TryExpr, syntax.FailExpr:
		return true
	case syntax.Lambda:
		return false
	}
	for ch := range c.t.View(n).Children() {
		if c.hasLeave(ch.ID()) {
			return true
		}
	}
	return false
}
