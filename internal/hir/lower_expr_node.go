package hir

import "kigumi/internal/syntax"

// A block already carries its own coercion on Block.Coercion (set by
// l.block for the same node), so the wrapping BlockExpr must not record
// it a second time.
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
