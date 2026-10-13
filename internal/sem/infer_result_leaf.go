package sem

import "kigumi/internal/syntax"

func (c *checker) resultLeaves(body syntax.NodeID) []syntax.NodeID {
	return append(c.tailLeaves(body, 0), c.retLeaves...)
}

func (c *checker) tailLeaves(n syntax.NodeID, depth int) []syntax.NodeID {
	if n == 0 || depth > 16 {
		return nil
	}
	switch c.t.Kind(n) {
	case syntax.Paren:
		return c.tailLeaves(syntax.NodeID(c.t.Nodes[n].Lhs), depth+1)
	case syntax.Block:
		stmts := c.t.Children(n)
		if len(stmts) == 0 || c.t.Kind(stmts[len(stmts)-1]) != syntax.ExprStmt {
			return nil
		}
		return c.tailLeaves(syntax.NodeID(c.t.Nodes[stmts[len(stmts)-1]].Lhs), depth+1)
	case syntax.IfExpr:
		s := ifExpr(c.t, n)
		return append(c.tailLeaves(s.Then, depth+1), c.tailLeaves(s.Else, depth+1)...)
	case syntax.MatchExpr:
		var out []syntax.NodeID
		for _, arm := range c.t.Children(syntax.NodeID(c.t.Nodes[n].Rhs)) {
			out = append(out, c.tailLeaves(matchArm(c.t, arm).Body, depth+1)...)
		}
		return out
	case syntax.ReturnExpr:
		return nil
	}
	return []syntax.NodeID{n}
}

func (c *checker) stripParen(n syntax.NodeID) syntax.NodeID {
	for c.t.Kind(n) == syntax.Paren {
		n = syntax.NodeID(c.t.Nodes[n].Lhs)
	}
	return n
}
