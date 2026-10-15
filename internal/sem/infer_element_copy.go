package sem

import "kigumi/internal/syntax"

// elementCopy: `xs[i] = ys[j]` between FixedArray elements of an in-place type fills the slot's own box.
func (c *checker) elementCopy(lhs, rhs syntax.NodeID, t TypeID) bool {
	t = c.vars.resolve(t)
	return c.r.InPlaceCopy(t) && c.fixedElementOf(lhs, t) && c.fixedElementOf(c.stripParen(rhs), t)
}

func (c *checker) fixedElementOf(n syntax.NodeID, t TypeID) bool {
	if c.t.Kind(n) != syntax.BracketExpr {
		return false
	}
	bt := c.vars.resolve(c.info.Types[syntax.NodeID(c.t.Nodes[n].Lhs)])
	for c.r.Types.Kind(bt) == KRef {
		bt = c.vars.resolve(c.r.Types.Node(bt).Elem)
	}
	elem, _, ok := c.r.FixedElem(bt)
	return ok && c.vars.resolve(elem) == t
}
