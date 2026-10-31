package sem

import "kigumi/internal/compiler/syntax"

type captureUse struct {
	node syntax.NodeID
	ent  EntityID
}

// deferCaptureReserve queues a use of a reserved binding inside a lambda that keeps it; settleCaptureReserves judges it once the whole body is typed.
func (c *checker) deferCaptureReserve(ctx *lambdaCtx, n syntax.NodeID, ent EntityID) {
	if len(c.reserved[ent]) == 0 || ctx.immediate || c.copyNow(c.r.Entities[ent].Type) {
		return
	}
	ctx.reserveUses = append(ctx.reserveUses, captureUse{node: n, ent: ent})
}

func (c *checker) settleCaptureReserves(ctx *lambdaCtx) {
	lambda := c.r.Entities[ctx.ent].Node
	var reads []heldUse
	for _, u := range ctx.reserveUses {
		p, top, above := c.captureChain(lambda, u)
		i, held := c.reservedOver(p.ent, p.fields)
		if !held {
			continue
		}
		if c.cleanup == 0 && c.readOnlyUse(top, above) {
			reads = append(reads, heldUse{node: u.node, res: c.reserved[p.ent][i]})
			continue
		}
		if !c.mutBorrowOperand(above) {
			c.conflictUse(u.node, p, false)
		}
	}
	c.settleReadOnlyLambda(lambda, reads)
	c.noteClosureWrites(ctx)
}

func (c *checker) captureChain(lambda syntax.NodeID, u captureUse) (reservedPlace, syntax.NodeID, []syntax.NodeID) {
	p := reservedPlace{ent: u.ent}
	path := c.pathTo(lambda, u.node)
	cur := u.node
	i := len(path) - 1
	for ; i >= 0; i-- {
		par := path[i]
		if c.t.Kind(par) == syntax.Paren {
			cur = par
			continue
		}
		if c.t.Kind(par) != syntax.MemberExpr || c.t.View(par).AsMember().Base().ID() != cur {
			break
		}
		fld := c.info.Uses[par]
		if fld == 0 || c.r.Entities[fld].Kind != EntField {
			break
		}
		p.fields = append(p.fields, fld)
		cur = par
	}
	return p, cur, path[:i+1]
}

func (c *checker) pathTo(root, target syntax.NodeID) []syntax.NodeID {
	if root == target || !c.within(target, root) {
		return nil
	}
	for ch := range c.t.View(root).Children() {
		if ch.ID() == target {
			return []syntax.NodeID{root}
		}
		if sub := c.pathTo(ch.ID(), target); sub != nil {
			return append([]syntax.NodeID{root}, sub...)
		}
	}
	return nil
}

// A move capture inside a cleanup consumes the binding when the closure is built, so the cleanup reserves it.
func (c *checker) reserveCleanupCaptures(n syntax.NodeID, ctx *lambdaCtx) {
	if c.cleanup == 0 {
		return
	}
	for _, cp := range ctx.list {
		if cp.Mode != capMove || cp.OwnCell || !c.ownsValue(cp.Local) || c.declaredIn(cp.Local, c.cleanupNode) {
			continue
		}
		p := reservedPlace{ent: cp.Local, node: n}
		c.consumeAfterAssign(n, p)
		c.cleanupLog.consumed = append(c.cleanupLog.consumed, p)
	}
}
