package sem

import "kigumi/internal/syntax"

func (c *checker) cleanupStmt(n syntax.NodeID) {
	if c.t.Kind(n) == syntax.DeferFailStmt && !c.isResultFn() {
		c.errAt(n, cDeferFailNoResult)
	}
	outerNode, outerLog := c.cleanupNode, c.cleanupLog
	c.cleanupNode, c.cleanupLog.consumed, c.cleanupLog.assigned = n, nil, nil
	c.cleanup++
	outerLoop, outerBreaks := c.loop, c.breaks
	c.loop, c.breaks = 0, nil
	body := c.cleanupBody(n)
	if c.t.Kind(body) == syntax.Block {
		c.check(body, TyUnit)
	} else {
		c.stmt(body)
	}
	c.cleanup--
	c.loop, c.breaks = outerLoop, outerBreaks
	consumed, assigned := c.cleanupLog.consumed, c.cleanupLog.assigned
	earlier := c.cleanupLog.earlier
	c.cleanupNode, c.cleanupLog = outerNode, outerLog
	c.cleanupLog.earlier = earlier
	c.holdReservations(n, consumed)
	c.rejectLoopConsume(n, consumed)
	for _, a := range assigned {
		a.scope = c.scope
		c.cleanupLog.earlier = append(c.cleanupLog.earlier, a)
	}
}

func (c *checker) cleanupBody(n syntax.NodeID) syntax.NodeID {
	if c.t.Kind(n) == syntax.DeferFailStmt {
		return c.t.View(n).AsDeferFail().Body().ID()
	}
	return c.t.View(n).AsDefer().Body().ID()
}
