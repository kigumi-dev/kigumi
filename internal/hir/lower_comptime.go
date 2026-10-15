package hir

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

func (l *lowerer) comptime(n, body syntax.NodeID) *Expr {
	out := l.node(n, Comptime)
	inner := l.expr(body)
	l.adoptUntyped(inner, out.Type)
	out.Args = []*Expr{inner}
	return out
}

// adoptUntyped gives a block's untyped literal result the type the block is used at, so a wide value is not cut to the default width.
func (l *lowerer) adoptUntyped(e *Expr, want sem.TypeID) {
	if !l.r.Types.IsNumeric(want) {
		return
	}
	switch e.Kind {
	case Lit:
		if t := l.info.Types[e.Node]; t == 0 || l.r.Types.Kind(t) == sem.KUntyped {
			e.Type = want
		}
	case BlockExpr:
		if e.Block.Tail != nil && l.r.Types.Kind(e.Type) == sem.KUntyped {
			l.adoptUntyped(e.Block.Tail, want)
			e.Type, e.Block.Type = want, want
		}
	case Wrap:
		if len(e.Args) == 1 && l.r.Types.Kind(e.Type) == sem.KUntyped {
			l.adoptUntyped(e.Args[0], want)
			e.Type = want
		}
	}
}
