package syntax

import "kigumi/internal/lang/token"

func (p *parser) contextKeyDecl(pf prefix) NodeID {
	tok := p.advance()
	name := p.expect(token.Ident)
	p.expect(token.Colon)
	typ := p.typ()
	return p.tree.addRec(ContextKeyDecl, tok, []uint32{pf.doc, uint32(p.attrList(pf)), uint32(pf.vis), name, uint32(typ)})
}
