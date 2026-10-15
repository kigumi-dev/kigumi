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
		return c.tailLeaves(c.t.View(n).AsParen().Inner().ID(), depth+1)
	case syntax.Block:
		tail := c.t.View(n).AsBlock().Tail()
		if !tail.Valid() {
			return nil
		}
		return c.tailLeaves(tail.ID(), depth+1)
	case syntax.IfExpr:
		s := ifExpr(c.t, n)
		return append(c.tailLeaves(s.Then, depth+1), c.tailLeaves(s.Else, depth+1)...)
	case syntax.MatchExpr:
		var out []syntax.NodeID
		for arm := range c.t.View(n).AsMatch().Arms().IDs() {
			out = append(out, c.tailLeaves(matchArm(c.t, arm).Body, depth+1)...)
		}
		return out
	case syntax.ReturnExpr:
		return nil
	}
	return []syntax.NodeID{n}
}

func (c *checker) stripParen(n syntax.NodeID) syntax.NodeID {
	return c.t.Unparen(n)
}
