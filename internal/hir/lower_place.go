package hir

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// place resolves an assignment target; its operands are lowered here so
// the builder evaluates them once, before the right-hand side.
func (l *lowerer) place(lhs syntax.NodeID) *Place {
	v := l.t.View(lhs)
	out := &Place{Node: lhs, Type: l.typeOf(lhs)}
	switch v.Kind() {
	case syntax.Paren:
		return l.place(v.AsParen().Inner().ID())
	case syntax.MemberExpr:
		base := v.AsMember().Base().ID()
		if ent := l.info.Uses[lhs]; ent != 0 && l.r.Entity(ent).Kind == sem.EntType {
			// `.value`: the same place as the receiver, just retyped.
			inner := *l.place(base)
			inner.Node, inner.Type = lhs, l.typeOf(lhs)
			return &inner
		}
		out.Kind, out.Base, out.Index = PlaceField, l.expr(base), l.r.Field(l.info.Uses[lhs]).Index
	case syntax.BracketExpr:
		b := v.AsBracket()
		out.Kind, out.Base, out.Idx = PlaceIndex, l.expr(b.Base().ID()), l.expr(b.Args().ItemID(0))
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
