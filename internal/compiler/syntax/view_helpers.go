package syntax

// Tail is the expression of the block's last statement when that statement
// is an expression statement; otherwise absent.
func (b BlockView) Tail() ExprView {
	if n := b.Len(); n > 0 && b.At(n-1).Kind() == ExprStmt {
		return b.At(n - 1).AsExprStmt().Expr()
	}
	return ExprView{}
}

// ArgValue is the expression an argument or record entry stands for: the operand of a
// spread, named argument or field initializer, the node itself otherwise.
func (v View) ArgValue() ExprView {
	switch v.Kind() {
	case Spread:
		return v.AsSpread().Value()
	case NamedArg:
		return v.AsNamedArg().Value()
	case FieldInit:
		return v.AsFieldInit().Value()
	}
	return ExprView{v}
}

// AddPath appends a synthetic Path node over the given identifier tokens,
// for names the checker resolves that the source never spelled as a path.
func (t *Tree) AddPath(first uint32, segs []uint32) NodeID {
	start := uint32(len(t.extra))
	t.extra = append(t.extra, segs...)
	return t.add(rawNode{Kind: Path, Tok: first, Lhs: start, Rhs: uint32(len(t.extra))})
}

// Base is the expression a Paren, Member, OptMember, Bracket or Try node
// wraps or selects from; absent for any other kind.
func (v View) Base() ExprView {
	switch v.Kind() {
	case Paren:
		return v.AsParen().Inner().AsExpr()
	case MemberExpr:
		return v.AsMember().Base()
	case OptMemberExpr:
		return v.AsOptMember().Base()
	case BracketExpr:
		return v.AsBracket().Base()
	case TryExpr:
		return v.AsTry().Operand()
	}
	return ExprView{}
}

// Unparen strips the parentheses around id.
func (t *Tree) Unparen(id NodeID) NodeID {
	for t.Kind(id) == Paren {
		id = t.View(id).AsParen().Inner().ID()
	}
	return id
}

// NodeCount is the number of node ids in use, counting the absent node 0.
func (t *Tree) NodeCount() int { return len(t.nodes) }

// AddPathPrefix appends a synthetic Path node over all but the last segment
// of path, for resolving the qualifier of `a.b.C`.
func (t *Tree) AddPathPrefix(path NodeID) NodeID {
	n := t.nodes[path]
	return t.AddPath(t.extra[n.Lhs], t.extra[n.Lhs:n.Rhs-1])
}
