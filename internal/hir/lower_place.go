package hir

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// place resolves an assignment target; its operands are lowered here so
// the builder evaluates them once, before the right-hand side.
func (l *lowerer) place(lhs syntax.NodeID) *Place {
	node := l.t.Nodes[lhs]
	out := &Place{Node: lhs, Type: l.typeOf(lhs)}
	switch node.Kind {
	case syntax.Paren:
		return l.place(syntax.NodeID(node.Lhs))
	case syntax.MemberExpr:
		if ent := l.info.Uses[lhs]; ent != 0 && l.r.Entity(ent).Kind == sem.EntType {
			// `.value`: the same place as the receiver, just retyped.
			inner := *l.place(syntax.NodeID(node.Lhs))
			inner.Node, inner.Type = lhs, l.typeOf(lhs)
			return &inner
		}
		out.Kind, out.Base, out.Index = PlaceField, l.expr(syntax.NodeID(node.Lhs)), l.r.Field(l.info.Uses[lhs]).Index
	case syntax.BracketExpr:
		out.Kind, out.Base, out.Idx = PlaceIndex, l.expr(syntax.NodeID(node.Lhs)), l.expr(l.t.Children(syntax.NodeID(node.Rhs))[0])
	default:
		ent := l.info.Uses[lhs]
		if ent != 0 && l.r.Entity(ent).Kind == sem.EntStatic {
			out.Kind, out.Ent = PlaceGlobal, ent
			return out
		}
		out.Kind, out.Ent, out.Expr = PlaceLocal, ent, l.expr(lhs)
	}
	return out
}
