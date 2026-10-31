package sem

import "kigumi/internal/compiler/syntax"

// rejectTempBorrowLet stops `let k = &mk()` (also through a field, `?.` field or index of the temporary)
// when the temporary owns something its end-of-statement drop would free under k; `x?.f`, a block, `if` and `match` build fresh temporaries.
func (c *checker) rejectTempBorrowLet(init syntax.NodeID) {
	if n, root, t, ok := c.tempBorrow(init); ok {
		c.errAt(n, cTempBorrowLet, c.temporaryName(root, t))
	}
}

// rejectTempBorrowStore is rejectTempBorrowLet for `place = &mk()`.
func (c *checker) rejectTempBorrowStore(rhs syntax.NodeID) {
	if n, root, t, ok := c.tempBorrow(rhs); ok {
		c.errAt(n, cTempBorrowStore, c.temporaryName(root, t))
	}
}

func (c *checker) tempBorrow(init syntax.NodeID) (syntax.NodeID, syntax.NodeID, TypeID, bool) {
	n := c.t.Unparen(init)
	for c.t.Kind(n) == syntax.Block {
		tail := c.t.View(n).AsBlock().Tail()
		if !tail.Valid() {
			return 0, 0, 0, false
		}
		n = c.t.Unparen(tail.ID())
	}
	if c.t.Kind(n) != syntax.BorrowExpr {
		return 0, 0, 0, false
	}
	operand := c.t.View(n).AsBorrow().Operand().ID()
	root, opt := operand, false
	for {
		root = c.t.Unparen(root)
		switch c.t.Kind(root) {
		case syntax.MemberExpr:
			root = c.t.View(root).AsMember().Base().ID()
			continue
		case syntax.OptMemberExpr:
			root, opt = c.t.View(root).AsOptMember().Base().ID(), true
			continue
		case syntax.BracketExpr:
			root = c.t.View(root).AsBracket().Base().ID()
			continue
		case syntax.CallExpr, syntax.RecordLit, syntax.TupleLit, syntax.StringLit, syntax.ByteStringLit, syntax.Block, syntax.IfExpr, syntax.IfLetExpr, syntax.MatchExpr:
		default:
			if !opt {
				return 0, 0, 0, false
			}
			root = c.t.Unparen(operand)
		}
		break
	}
	t := c.vars.resolve(c.info.Types[root])
	switch {
	case t == TyPoison || t == 0 || t == TyNever || c.r.Types.Kind(t) == KRef:
		return 0, 0, 0, false
	case c.r.Types.Kind(t) == KPrim && t != TyString && t != TyBytes:
		return 0, 0, 0, false
	}
	return n, root, t, true
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
