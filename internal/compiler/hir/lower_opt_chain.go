package hir

import (
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// optChain lowers `a?.m(x)` as `match a { Some(q) -> Some(q.m(x)), _ -> None }`,
// matching `&a` when the checker bound the payload by reference.
func (l *lowerer) optChain(n syntax.NodeID, ch *sem.OptChain) *Expr {
	tt := l.r.Types
	baseType := l.typeOf(ch.Base)
	if ch.Through {
		baseType = tt.Node(baseType).Elem
	}
	var some, none sem.EntityID
	for _, v := range l.r.TypeDecl(tt.OptionEnt()).Variants {
		switch l.r.Entity(v).Name {
		case "Some":
			some = v
		case "None":
			none = v
		}
	}
	scrutinee := l.expr(ch.Base)
	if ch.ByRef && !ch.Through {
		borrow := l.node(n, Borrow)
		borrow.Type, borrow.Args = tt.Ref(baseType, false), []*Expr{scrutinee}
		scrutinee = borrow
	}
	bind := &Pat{Kind: PatBind, Type: ch.Elem, Node: n, Ent: ch.Local, Borrow: ch.ByRef}
	hit := &Pat{Kind: PatCtor, Type: baseType, Node: n, Ent: some, Subs: []*Pat{bind}, Borrow: ch.ByRef}
	wrap := l.node(n, Variant)
	wrap.Ent, wrap.Args = some, []*Expr{l.expr(ch.Body)}
	miss := l.node(n, Variant)
	miss.Ent = none
	out := l.node(n, Match)
	out.Args = []*Expr{scrutinee}
	out.Arms = []*Arm{
		{Pat: hit, Body: wrap},
		{Pat: &Pat{Kind: PatWild, Type: baseType, Node: n}, Body: miss},
	}
	return out
}
