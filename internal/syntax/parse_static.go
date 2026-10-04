package syntax

import "kigumi/internal/token"

// staticDeclAhead disambiguates the contextual `static` keyword from an
// identifier named "static" used as an ordinary expression.
func (p *parser) staticDeclAhead() bool {
	return p.peek(1) == token.KwMut || (p.peek(1) == token.Ident && p.peek(2) == token.Colon)
}

func (p *parser) staticDecl(pf prefix) NodeID {
	tok := p.advance()
	var flags uint32
	if p.eat(token.KwMut) {
		flags |= FlagMut
	}
	name := p.expect(token.Ident)
	p.expect(token.Colon)
	typ := p.typ()
	var value NodeID
	if p.eat(token.Assign) {
		value = p.expr()
	}
	return p.tree.addRec(StaticDecl, tok, []uint32{
		pf.doc, uint32(p.attrList(pf)), flags, uint32(pf.vis), name, uint32(typ), uint32(value),
	})
}
