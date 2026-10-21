package syntax

// AddIdentAt appends a synthetic identifier over token tok; the checker binds it itself.
func (t *Tree) AddIdentAt(tok uint32) NodeID {
	return t.add(rawNode{Kind: Ident, Tok: tok})
}

// Rebased appends a copy of the postfix node n (a member, call or index) that sits on base instead.
func (t *Tree) Rebased(n, base NodeID) NodeID {
	raw := t.nodes[n]
	raw.Lhs = uint32(base)
	return t.add(raw)
}

// OptMemberAsMember appends the plain member node `base.name` for the `?.` node o.
func (t *Tree) OptMemberAsMember(o, base NodeID) NodeID {
	raw := t.nodes[o]
	raw.Kind, raw.Lhs = MemberExpr, uint32(base)
	return t.add(raw)
}
