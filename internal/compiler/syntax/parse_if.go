package syntax

import "kigumi/internal/lang/token"

func (p *parser) ifExpr() NodeID {
	if !p.enterDepth() {
		return 0
	}
	defer p.exitDepth()
	return p.ifRest(p.expect(token.KwIf))
}

func (p *parser) ifRest(tok uint32) NodeID {
	if p.at(token.KwLet) {
		return p.ifLet(tok)
	}
	cond := p.condition()
	then := p.block()
	return p.tree.addRec(IfExpr, tok, []uint32{uint32(cond), uint32(then), uint32(p.elseClause())})
}

// ifLet parses `if let pattern = expr; guard { ... }`; a refutable pattern takes the else branch when it does not match.
func (p *parser) ifLet(tok uint32) NodeID {
	p.expect(token.KwLet)
	var flags uint32
	if p.eat(token.KwMut) {
		flags |= FlagMut
	}
	pat := p.pattern()
	p.expect(token.Assign)
	p.skipNewlinesAfterAssign()
	init := p.condition()
	var guard NodeID
	if p.eat(token.Semicolon) {
		guard = p.condition()
	}
	then := p.block()
	return p.tree.addRec(IfLetExpr, tok, []uint32{flags, uint32(pat), uint32(init), uint32(guard), uint32(then), uint32(p.elseClause())})
}

func (p *parser) elseClause() NodeID {
	if p.at(token.Newline) && p.peekPastNewlines() == token.KwElse {
		p.skipNewlines()
	}
	if p.atIdent("elif") {
		tok := p.pos
		p.elifError()
		return p.ifRest(tok)
	}
	if !p.eat(token.KwElse) {
		return 0
	}
	if p.at(token.KwIf) {
		return p.ifExpr()
	}
	return p.block()
}
