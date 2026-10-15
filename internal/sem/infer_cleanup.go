package sem

import "kigumi/internal/syntax"

func (c *checker) cleanupStmt(n syntax.NodeID) {
	if c.t.Kind(n) == syntax.ErrdeferStmt && !c.isResultFn() {
		c.errAt(n, cErrdeferOutsideResult)
	}
	c.cleanup++
	body := c.cleanupBody(n)
	if c.t.Kind(body) == syntax.Block {
		c.check(body, TyUnit)
	} else {
		c.stmt(body)
	}
	c.cleanup--
}

func (c *checker) cleanupBody(n syntax.NodeID) syntax.NodeID {
	if c.t.Kind(n) == syntax.ErrdeferStmt {
		return c.t.View(n).AsErrdefer().Body().ID()
	}
	return c.t.View(n).AsDefer().Body().ID()
}
