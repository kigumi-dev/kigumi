package hir

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

func (l *lowerer) pattern(p syntax.NodeID) *Pat {
	v := l.t.View(p)
	out := &Pat{Type: l.typeOf(p), Node: p, Borrow: l.info.Pats[p].Borrow, Mut: l.info.Pats[p].Mut}
	switch v.Kind() {
	case syntax.PatWildcard:
		out.Kind = PatWild
	case syntax.PatBind:
		out.Kind, out.Ent = PatBind, l.info.Defs[p]
	case syntax.PatLit:
		out.Kind, out.Lit = PatLit, l.expr(v.AsPatLit().Lit().ID())
	case syntax.PatCtor:
		if ent := l.info.Uses[p]; l.r.Entity(ent).Kind == sem.EntConst {
			out.Kind, out.Lit = PatLit, l.constLit(p, ent)
			return out
		}
		out.Kind, out.Ent = PatCtor, l.info.Uses[p]
		out.IsType = l.r.Entity(out.Ent).Kind != sem.EntVariant
		for s := range v.AsPatCtor().Args().IDs() {
			out.Subs = append(out.Subs, l.pattern(s))
		}
	case syntax.PatRecord:
		out.Kind, out.Ent = PatRecord, l.info.Uses[p]
		out.Variant = l.r.Entity(out.Ent).Kind == sem.EntVariant
		for f := range v.AsPatRecord().Fields().IDs() {
			out.Fields = append(out.Fields, l.patField(f, out.Ent))
		}
	case syntax.PatTuple:
		out.Kind, out.Ent = PatRecord, l.info.Uses[p]
		for i, sv := range v.AsPatTuple().Enumerate() {
			s := sv.ID()
			out.Fields = append(out.Fields, &PatField{Node: s, Type: l.typeOf(s), Index: i, Sub: l.pattern(s)})
		}
	case syntax.PatOr:
		out.Kind = PatOr
		for alt := range v.AsPatOr().IDs() {
			out.Subs = append(out.Subs, l.pattern(alt))
		}
	case syntax.PatRange:
		r := v.AsPatRange()
		out.Kind = PatRange
		out.Lo, out.Hi = l.expr(r.Lo().ID()), l.expr(r.Hi().ID())
		out.Inclusive = r.Op().Kind() == token.DotDotEq
	default:
		out.Kind = PatNever
	}
	return out
}

// patField resolves the field a record-pattern entry names: a payload
// position of a variant, or a field index of a record.
func (l *lowerer) patField(f syntax.NodeID, owner sem.EntityID) *PatField {
	fv := l.t.View(f).AsPatField()
	name := fv.Name().Text()
	out := &PatField{Node: f, Type: l.typeOf(f)}
	if sub := fv.Sub(); sub.Valid() {
		out.Sub = l.pattern(sub.ID())
	} else {
		out.Bind = l.info.Defs[f]
	}
	if l.r.Entity(owner).Kind == sem.EntVariant {
		for i, n := range l.r.Variant(owner).Names {
			if n == name {
				out.Payload, out.Index = true, i
				return out
			}
		}
	}
	out.Index = l.r.Field(l.r.FindField(owner, name)).Index
	return out
}

func (l *lowerer) constLit(p syntax.NodeID, ent sem.EntityID) *Expr {
	out := l.node(p, Lit)
	out.Lit, _ = l.r.ConstValueOf(ent)
	return out
}
