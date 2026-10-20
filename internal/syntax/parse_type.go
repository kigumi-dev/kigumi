package syntax

import "kigumi/internal/token"

// typ parses a type expression. `T?` and `T!` are postfix; a `!=` token in
// type position is split so `x: Ast!= y` still works.
func (p *parser) typ() NodeID {
	if !p.enterDepth() {
		return 0
	}
	defer p.exitDepth()
	return p.typPostfix(p.typPrimary())
}

func (p *parser) typPostfix(t NodeID) NodeID {
	for {
		switch {
		case p.at(token.Question) && !p.tok().HasSpaceBefore():
			t = p.node1(TypeOptional, p.advance(), t)
		case p.at(token.Bang) && !p.tok().HasSpaceBefore():
			t = p.node1(TypeResult, p.advance(), t)
		case p.at(token.NotEq) && !p.tok().HasSpaceBefore():
			t = p.node1(TypeResult, p.advance(), t)
			p.splitAssign = true
			return t
		default:
			return t
		}
	}
}

func (p *parser) typPrimary() NodeID {
	switch {
	case p.at(token.KwPure), p.at(token.KwNoalloc), p.at(token.KwUnsafe), p.at(token.KwFn),
		p.at(token.KwAsync):
		return p.fnType(false)
	case p.at(token.KwExtern):
		return p.fnType(false)
	case p.at(token.Star):
		tok := p.advance()
		var flags uint32
		switch {
		case p.eat(token.KwConst):
		case p.eat(token.KwMut):
			flags = FlagMut
		default:
			p.errorf("expected `const` or `mut` after `*` in pointer type")
		}
		return p.nodeFlag(TypePtr, tok, flags, p.typ())
	case p.at(token.Amp):
		tok := p.advance()
		// A lifetime tag replaces the `&` as the node's token: existing
		// callers that read it back only care about its text when its
		// kind is Lifetime, so a plain `&T` is unaffected.
		if p.at(token.Lifetime) {
			tok = p.advance()
		}
		var flags uint32
		if p.eat(token.KwMut) {
			flags = FlagMut
		}
		return p.nodeFlag(TypeRef, tok, flags, p.typ())
	case p.at(token.LParen):
		return p.parenOrTupleType()
	case p.at(token.Ident):
		path := p.path()
		var args NodeID
		if p.at(token.LBracket) && !p.tok().HasSpaceBefore() {
			args = p.typeArgs()
		}
		return p.node2(TypePath, p.tree.nodes[path].Tok, path, args)
	case p.at(token.Int), p.at(token.KwTrue), p.at(token.KwFalse):
		// A const generic argument: `FixedArray[u8, 32]`.
		return p.literal()
	case p.at(token.Lifetime):
		// A lifetime argument: `View['a]`.
		return p.leaf(TypeLifetime, p.advance())
	}
	p.errorf("expected type, found %s", p.describe())
	return 0
}

// parenOrTupleType parses `(T)`, a grouped type, and `(A, B, ...)`, a tuple
// type: a comma after the first element is what commits to a tuple.
func (p *parser) parenOrTupleType() NodeID {
	tok := p.advance()
	p.skipNewlines()
	first := p.typ()
	p.skipNewlines()
	if !p.at(token.Comma) {
		p.expect(token.RParen)
		return p.node1(Paren, tok, first)
	}
	items := []NodeID{first}
	for p.eat(token.Comma) {
		p.skipNewlines()
		if p.at(token.RParen) {
			break
		}
		items = append(items, p.typ())
		p.skipNewlines()
	}
	p.expect(token.RParen)
	return p.tree.addList(TypeTuple, tok, items)
}

func (p *parser) typeArgs() NodeID {
	var items []NodeID
	tok := p.layoutList(token.LBracket, token.RBracket, func() {
		items = append(items, p.typ())
	})
	return p.tree.addList(List, tok, items)
}
