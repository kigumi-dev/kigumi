package sem

import "kigumi/internal/syntax"

// rejectTempBorrowLet stops `let k = &mk()` (also through a field, `?.` field or index of the temporary)
// when the temporary owns something its end-of-statement drop would free under k.
func (c *checker) rejectTempBorrowLet(init syntax.NodeID) {
	n := c.t.Unparen(init)
	for c.t.Kind(n) == syntax.Block {
		tail := c.t.View(n).AsBlock().Tail()
		if !tail.Valid() {
			return
		}
		n = c.t.Unparen(tail.ID())
	}
	if c.t.Kind(n) != syntax.BorrowExpr {
		return
	}
	root := c.t.View(n).AsBorrow().Operand().ID()
	for {
		root = c.t.Unparen(root)
		switch c.t.Kind(root) {
		case syntax.MemberExpr:
			root = c.t.View(root).AsMember().Base().ID()
			continue
		case syntax.OptMemberExpr:
			root = c.t.View(root).AsOptMember().Base().ID()
			continue
		case syntax.BracketExpr:
			root = c.t.View(root).AsBracket().Base().ID()
			continue
		case syntax.CallExpr, syntax.RecordLit, syntax.TupleLit, syntax.StringLit, syntax.ByteStringLit:
		default:
			return
		}
		break
	}
	t := c.vars.resolve(c.info.Types[root])
	switch {
	case t == TyPoison || t == 0 || t == TyNever || c.r.Types.Kind(t) == KRef:
		return
	case c.r.Types.Kind(t) == KPrim && t != TyString && t != TyBytes:
		return
	}
	c.errAt(n, cTempBorrowLet, c.temporaryName(root, t))
}

func (c *checker) temporaryName(root syntax.NodeID, t TypeID) string {
	if c.t.Kind(root) == syntax.CallExpr {
		callee := c.t.Unparen(c.t.View(root).AsCall().Callee().ID())
		if k := c.t.Kind(callee); k == syntax.Ident || k == syntax.MemberExpr {
			sp := c.t.Span(callee)
			return "the result of `" + string(c.t.File.Src[sp.Start:sp.End]) + "(..)`"
		}
	}
	return "a temporary `" + c.r.TypeString(t) + "`"
}
