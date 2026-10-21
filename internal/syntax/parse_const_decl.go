package syntax

import "kigumi/internal/token"

func (p *parser) constDecl(pf prefix) NodeID {
	tok := p.expect(token.KwConst)
	name := p.expect(token.Ident)
	var typ NodeID
	if p.eat(token.Colon) {
		typ = p.typ()
	}
	p.expect(token.Assign)
	p.skipNewlinesAfterAssign()
	value := p.expr()
	return p.tree.addRec(ConstDecl, tok, []uint32{pf.doc, uint32(p.attrList(pf)), uint32(pf.vis), name, uint32(typ), uint32(value), uint32(p.initThunk(name, typ, value))})
}

func (p *parser) attrList(pf prefix) NodeID {
	if len(pf.attrs) == 0 {
		return 0
	}
	return p.tree.addList(List, p.tree.nodes[pf.attrs[0]].Tok, pf.attrs)
}
