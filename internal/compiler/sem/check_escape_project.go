package sem

import "kigumi/internal/compiler/syntax"

func (c *checker) projectionAlias(expr syntax.NodeID) EntityID {
	v := c.t.View(expr)
	var base syntax.NodeID
	switch c.t.Kind(expr) {
	case syntax.MemberExpr:
		base = v.AsMember().Base().ID()
	case syntax.OptMemberExpr:
		base = v.AsOptMember().Base().ID()
	case syntax.BracketExpr:
		base = v.AsBracket().Base().ID()
	case syntax.Unary:
		base = v.AsUnary().Operand().ID()
	case syntax.BorrowExpr:
		base = v.AsBorrow().Operand().ID()
	case syntax.MoveExpr:
		base = v.AsMove().Operand().ID()
	case syntax.TryExpr:
		base = v.AsTry().Operand().ID()
	}
	if base == 0 {
		return 0
	}
	p := c.restrictedAliasOf(base)
	if p == 0 || !c.exprMayHoldFn(expr) {
		return 0
	}
	return p
}

func (c *checker) exprMayHoldFn(expr syntax.NodeID) bool {
	t := c.info.Types[expr]
	return t == 0 || c.r.mayContainFn(c.vars.resolve(t), map[TypeID]bool{})
}

func (c *checker) holdsFn(expr syntax.NodeID, p EntityID) bool {
	ent := c.info.Uses[expr]
	return ent == p || c.exprMayHoldFn(expr)
}
