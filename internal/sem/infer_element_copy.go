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
	bt := c.vars.resolve(c.info.Types[c.t.View(n).AsBracket().Base().ID()])
	for c.r.Types.Kind(bt) == KRef {
		bt = c.vars.resolve(c.r.Types.Node(bt).Elem)
	}
	elem, _, ok := c.r.FixedElem(bt)
	return ok && c.vars.resolve(elem) == t
}

// copyShape: both sides read a place through a projection, so the store may copy between two slots.
func (c *checker) copyShape(lhs, rhs syntax.NodeID) bool {
	rhs = c.stripParen(rhs)
	if !isProjection(c.t.Kind(lhs)) || !isProjection(c.t.Kind(rhs)) {
		return false
	}
	if c.t.Kind(lhs) == syntax.BracketExpr && c.t.Kind(rhs) == syntax.BracketExpr {
		return true
	}
	return c.projectedBase(lhs) && c.projectedBase(rhs)
}

func isProjection(k syntax.NodeKind) bool { return k == syntax.MemberExpr || k == syntax.BracketExpr }

func (c *checker) projectedBase(n syntax.NodeID) bool {
	return isProjection(c.t.Kind(c.stripParen(c.t.View(n).Base().ID())))
}

// fieldCopy: `a.f = b.g` or a slot written from a field, between places of an in-place type, fills the destination's own box.
func (c *checker) fieldCopy(lhs, rhs syntax.NodeID, t TypeID) bool {
	t = c.vars.resolve(t)
	rhs = c.stripParen(rhs)
	return c.r.InPlaceCopy(t) && c.projectedBase(lhs) && c.projectedBase(rhs) && c.placeOfType(lhs, t) && c.placeOfType(rhs, t)
}

func (c *checker) placeOfType(n syntax.NodeID, t TypeID) bool {
	if c.t.Kind(n) == syntax.BracketExpr {
		return c.fixedElementOf(n, t)
	}
	use := c.info.Uses[n]
	return use != 0 && c.r.Entities[use].Kind == EntField && c.vars.resolve(c.info.Types[n]) == t
}
