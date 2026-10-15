package syntax

import (
	"iter"

	"kigumi/internal/token"
)

// View is a node of a Tree: the tree and the node id, with no allocation.
// The zero View is the absent node; conversions to a kind view are unchecked.
type View struct {
	t  *Tree
	id NodeID
}

func (t *Tree) View(id NodeID) View { return View{t, id} }

func (v View) ID() NodeID       { return v.id }
func (v View) Tree() *Tree      { return v.t }
func (v View) Valid() bool      { return v.id != 0 }
func (v View) Kind() NodeKind   { return v.t.Kind(v.id) }
func (v View) Span() token.Span { return v.t.Span(v.id) }
func (v View) Tok() TokenRef    { return TokenRef{v.t, v.t.nodes[v.id].Tok} }

// Is reports whether the node's kind is one of kinds.
func (v View) Is(kinds ...NodeKind) bool {
	k := v.Kind()
	for _, w := range kinds {
		if k == w {
			return true
		}
	}
	return false
}

func (v View) at(id uint32) View { return View{v.t, NodeID(id)} }
func (v View) lhs() uint32       { return v.t.nodes[v.id].Lhs }
func (v View) rhs() uint32       { return v.t.nodes[v.id].Rhs }
func (v View) rec(i int) uint32 {
	n := v.t.nodes[v.id]
	return v.t.extra[n.Lhs:n.Rhs][i]
}

func (v View) tokRef(idx uint32) TokenRef { return TokenRef{v.t, idx} }

// TokenRef is a token of a tree; index 0 is the absent token.
type TokenRef struct {
	t   *Tree
	idx uint32
}

func (r TokenRef) Valid() bool        { return r.idx != 0 }
func (r TokenRef) Index() uint32      { return r.idx }
func (r TokenRef) Text() string       { return r.t.TokText(r.idx) }
func (r TokenRef) Span() token.Span   { return r.t.Toks[r.idx].Span }
func (r TokenRef) Kind() token.Kind   { return r.t.Toks[r.idx].Kind }
func (r TokenRef) Token() token.Token { return r.t.Toks[r.idx] }

// Class views group the kinds that can stand where an expression, statement,
// declaration, pattern or type expression is expected.
type (
	ExprView     struct{ View }
	StmtView     struct{ View }
	DeclView     struct{ View }
	PatternView  struct{ View }
	TypeExprView struct{ View }
)

func (v View) AsExpr() ExprView         { return ExprView{v} }
func (v View) AsStmt() StmtView         { return StmtView{v} }
func (v View) AsDecl() DeclView         { return DeclView{v} }
func (v View) AsPattern() PatternView   { return PatternView{v} }
func (v View) AsTypeExpr() TypeExprView { return TypeExprView{v} }

// All yields every node of the tree in creation order, children before their parents.
func (t *Tree) All() iter.Seq[View] {
	return func(yield func(View) bool) {
		for id := 1; id < len(t.nodes); id++ {
			if !yield(View{t, NodeID(id)}) {
				return
			}
		}
	}
}
