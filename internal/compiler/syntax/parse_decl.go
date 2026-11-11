package syntax

import "kigumi/internal/lang/token"

func (p *parser) parseFile() NodeID {
	var decls []NodeID
	p.skipNewlines()
	for !p.at(token.EOF) {
		start := p.pos
		if d := p.decl(); d != 0 {
			decls = append(decls, d)
		}
		if p.pos == start {
			if p.at(token.RBrace) {
				p.errorf("unmatched `}`")
			} else {
				p.errorf("unexpected %s", p.describe())
			}
			p.recover()
			if p.pos == start {
				p.pos++
			}
		}
		p.endOfStatement()
		p.skipNewlines()
	}
	return p.tree.addList(File, 0, decls)
}

// prefix holds the doc comment, attributes, visibility and modifiers that
// precede a declaration, in any order.
type prefix struct {
	doc   uint32
	attrs []NodeID
	vis   NodeID
	mods  uint32
	start uint32
	seen  bool
}

func (p *parser) parsePrefix() prefix {
	var pf prefix
	pf.start = p.pos
	for {
		switch {
		case p.at(token.DocComment):
			pf.doc = p.advance()
			p.skipNewlines()
		case p.at(token.At):
			pf.attrs = append(pf.attrs, p.attr())
			p.skipNewlines()
		case p.at(token.KwPub):
			v := p.visibility()
			switch {
			case pf.vis == 0:
				pf.vis = v
			case p.sameVisibility(pf.vis, v):
				p.warnRedundant(p.visibilitySpan(v), "duplicate visibility")
			default:
				p.errorAt(p.tree.Span(v), "conflicting visibility modifiers")
				pf.vis = v
			}
		case p.at(token.KwPure):
			tok := p.tok()
			p.advance()
			bit := ModPure
			if p.at(token.Question) && !p.tok().HasSpaceBefore() {
				p.advance()
				bit = ModPureVar
			}
			if pf.mods&(ModPure|ModPureVar) != 0 {
				p.warnRedundantModifier(tok.Span)
			}
			pf.mods |= bit
		case p.at(token.KwNoalloc), p.at(token.KwAsync), p.at(token.KwUnsafe):
			bit := modBit(p.tok().Kind)
			if pf.mods&bit != 0 {
				p.warnRedundantModifier(p.tok().Span)
			}
			pf.mods |= bit
			p.advance()
		default:
			pf.seen = p.pos != pf.start
			return pf
		}
	}
}

func modBit(k token.Kind) uint32 {
	switch k {
	case token.KwPure:
		return ModPure
	case token.KwNoalloc:
		return ModNoalloc
	case token.KwAsync:
		return ModAsync
	}
	return ModUnsafe
}

func (p *parser) attr() NodeID {
	at := p.expect(token.At)
	if p.atIdent("unsafeGuarantee") && p.peek(1) == token.LParen {
		return p.unsafeGuaranteeAttr(at)
	}
	path := p.path()
	var args NodeID
	if p.at(token.LParen) && !p.tok().HasSpaceBefore() {
		open := p.pos
		args = p.tree.addList(List, open, p.exprList())
	}
	return p.node2(Attr, at, path, args)
}

func (p *parser) visibility() NodeID {
	tok := p.expect(token.KwPub)
	if !p.at(token.LParen) || p.tok().HasSpaceBefore() {
		return p.tree.add(rawNode{Kind: Visibility, Tok: tok})
	}
	p.advance()
	kindTok := p.pos
	var path NodeID
	switch {
	case p.atIdent("self"):
		p.errorf("`pub(self)` is the same as no modifier; remove it (v2.6 B4)")
		p.advance()
	case p.atIdent("module"), p.atIdent("super"):
		p.advance()
	case p.at(token.KwIn):
		p.advance()
		path = p.path()
	default:
		p.errorf("expected `self`, `module`, `super` or `in path` after `pub(`")
	}
	p.expect(token.RParen)
	return p.tree.add(rawNode{Kind: Visibility, Tok: tok, Lhs: kindTok, Rhs: uint32(path)})
}

func (p *parser) decl() NodeID {
	pf := p.parsePrefix()
	if pf.mods != 0 && !p.at(token.KwFn) && !p.at(token.LBrace) {
		p.errorf("`%s` is only allowed on `fn` declarations", modsString(pf.mods))
	}
	switch {
	case p.atIdent("static") && p.peek(1) == token.KwFn:
		p.errorf("`static fn` does not exist; an associated function is `fn Type.name(...)` without `self`")
		p.advance()
		return p.fnDecl(pf)
	case p.atIdent("static") && p.staticDeclAhead():
		return p.staticDecl(pf)
	case p.atIdent("impl") && p.peek(1) == token.Ident:
		p.errorf("`impl` blocks do not exist; write `fn Type.name(self)` at top level and use `@impl(Interface)` on the type")
		p.recover()
		return 0
	case p.atIdent("struct") && p.peek(1) == token.Ident:
		return p.foreignTypeDecl("struct", "write `type Name = { field Type }`; fields are `name Type` without a colon")
	case p.atIdent("enum") && p.peek(1) == token.Ident:
		return p.foreignTypeDecl("enum", "write `type Name = A | B(T)`; variants are separated by `|`")
	case p.at(token.KwFn):
		return p.fnDecl(pf)
	case p.at(token.KwType):
		return p.typeDecl(pf)
	case p.at(token.KwInterface):
		return p.interfaceDecl(pf)
	case p.at(token.KwConst):
		return p.constDecl(pf)
	case p.atIdent("context") && p.peek(1) == token.Ident && p.peek(2) == token.Colon:
		return p.contextKeyDecl(pf)
	case p.at(token.KwImport):
		return p.importDecl(pf)
	case p.atIdent("test") && p.peek(1) == token.String && p.testDeclAhead():
		return p.testDecl(pf)
	case p.at(token.KwExtern) || (p.atIdent("export") && p.peek(1) == token.LParen):
		return p.abiBlock(pf)
	case p.at(token.LBrace) && pf.mods == modBit(token.KwUnsafe) && pf.vis == 0 && len(pf.attrs) == 0:
		return p.node1(ExprStmt, pf.start, p.node1(UnsafeExpr, pf.start, p.block()))
	case p.at(token.LBrace) && pf.mods != 0:
		p.errorf("contract blocks were removed from declarations (v2.6 B2); write `%s` on each fn", modsString(pf.mods))
		return p.contractBlock(pf)
	case pf.seen:
		if pf.doc != 0 && pf.vis == 0 && pf.mods == 0 && len(pf.attrs) == 0 {
			return p.stmt()
		}
		p.errorf("expected declaration after modifiers, found %s", p.describe())
		p.recover()
		return 0
	}
	return p.stmt()
}

func (p *parser) rejectDeclPrefix(pf prefix, what string) {
	if pf.vis != 0 || pf.mods != 0 || len(pf.attrs) > 0 {
		p.errorAt(p.spanFrom(pf.start), "modifiers are not allowed on %s", what)
	}
}
