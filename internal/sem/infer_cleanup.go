package sem

import "kigumi/internal/syntax"

func (c *checker) cleanupStmt(n syntax.NodeID) {
	node := c.t.Nodes[n]
	if node.Kind == syntax.ErrdeferStmt && !c.isResultFn() {
		c.errAt(n, cErrdeferOutsideResult)
	}
	c.cleanup++
	body := syntax.NodeID(node.Lhs)
	if c.t.Kind(body) == syntax.Block {
		c.check(body, TyUnit)
	} else {
		c.stmt(body)
	}
	c.cleanup--
}
