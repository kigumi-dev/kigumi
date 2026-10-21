package syntax

// clone copies the subtree under id into fresh nodes, so one declaration's generics and types can be declared twice.
func (t *Tree) clone(id NodeID) NodeID {
	if id == 0 {
		return 0
	}
	n := t.nodes[id]
	switch kindTable[n.Kind].layout {
	case layL:
		n.Lhs = uint32(t.clone(NodeID(n.Lhs)))
	case layLR:
		n.Lhs = uint32(t.clone(NodeID(n.Lhs)))
		n.Rhs = uint32(t.clone(NodeID(n.Rhs)))
	case layFlagR, layTokR:
		n.Rhs = uint32(t.clone(NodeID(n.Rhs)))
	case layList:
		items := append([]uint32(nil), t.extra[n.Lhs:n.Rhs]...)
		for i, c := range items {
			items[i] = uint32(t.clone(NodeID(c)))
		}
		n.Lhs = uint32(len(t.extra))
		t.extra = append(t.extra, items...)
		n.Rhs = uint32(len(t.extra))
	case layRec:
		slots := append([]uint32(nil), t.extra[n.Lhs:n.Rhs]...)
		for _, i := range kindTable[n.Kind].children {
			slots[i] = uint32(t.clone(NodeID(slots[i])))
		}
		n.Lhs = uint32(len(t.extra))
		t.extra = append(t.extra, slots...)
		n.Rhs = uint32(len(t.extra))
	case layPath:
		toks := append([]uint32(nil), t.extra[n.Lhs:n.Rhs]...)
		n.Lhs = uint32(len(t.extra))
		t.extra = append(t.extra, toks...)
		n.Rhs = uint32(len(t.extra))
	}
	return t.add(n)
}
