package hir

import "kigumi/internal/compiler/syntax"

// l.block already records the coercion on Block.Coercion, so the wrapping BlockExpr must not.
func (l *lowerer) expr(n syntax.NodeID) *Expr {
	e := l.exprRaw(n)
	if e.Kind != BlockExpr {
		e.Coercion = l.coercion(n)
	}
	return e
}

func (l *lowerer) node(n syntax.NodeID, k ExprKind) *Expr {
	return &Expr{Kind: k, Type: l.typeOf(n), Loc: l.loc(n), Node: n}
}

func (l *lowerer) wrap(n, inner syntax.NodeID) *Expr {
	out := l.node(n, Wrap)
	out.Args = []*Expr{l.expr(inner)}
	return out
}
