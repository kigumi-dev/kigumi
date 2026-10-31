package syntax

// The thunk lets the checker and back ends run the initializer like a function body.
func (p *parser) initThunk(name uint32, typ, value NodeID) NodeID {
	if typ == 0 || value == 0 || p.tree.nodes[value].Kind != ComptimeExpr {
		return 0
	}
	tok := p.tree.nodes[value].Tok
	body := p.tree.addList(Block, tok, []NodeID{p.node1(ExprStmt, tok, value)})
	params := p.tree.addList(List, tok, nil)
	return p.tree.addRec(FnDecl, name, []uint32{0, 0, 0, 0, 0, name, 0, uint32(params), uint32(typ), uint32(body)})
}
