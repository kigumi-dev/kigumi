package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

// startsShared: a capture is a cell from the start when a write is already known, made by an argument evaluated after
// the lambda but checked before it, or made earlier in a loop around a closure that may outlive its call.
func (c *checker) startsShared(ctx *lambdaCtx, n syntax.NodeID, ent EntityID) bool {
	return c.r.Entities[ent].Flags&EfCapturedMut != 0 || c.writtenWithin(ent, ctx.later) || !ctx.immediate && c.writtenInLoopOf(ent, n)
}

// markLaterArgs keeps, for lambda argument a, the arguments evaluated after it, from the call's evaluation order.
func (c *checker) markLaterArgs(a syntax.NodeID, eval []syntax.NodeID) {
	i := slices.Index(eval, a)
	if i < 0 || i == len(eval)-1 {
		return
	}
	if c.t.Kind(a) == syntax.MoveExpr {
		a = c.t.Unparen(c.t.View(a).AsMove().Operand().ID())
	}
	if c.laterArgs == nil {
		c.laterArgs = map[syntax.NodeID][]syntax.NodeID{}
	}
	c.laterArgs[a] = eval[i+1:]
}

func (c *checker) writtenWithin(ent EntityID, nodes []syntax.NodeID) bool {
	for _, w := range c.held.localWrites[ent] {
		at := c.t.Span(w).Start
		if slices.ContainsFunc(nodes, func(a syntax.NodeID) bool { s := c.t.Span(a); return s.Start <= at && at < s.End }) {
			return true
		}
	}
	return false
}
