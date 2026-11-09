package syntax

import "kigumi/internal/lang/token"

func (p *parser) block() NodeID {
	if !p.enterDepth() {
		return 0
	}
	defer p.exitDepth()
	var stmts []NodeID
	tok := p.expect(token.LBrace)
	saved := p.noRecordLit
	p.noRecordLit = false
	p.skipNewlines()
	for !p.at(token.RBrace) && !p.at(token.EOF) {
		start := p.pos
		if s := p.stmt(); s != 0 {
			stmts = append(stmts, s)
		}
		if p.pos == start {
			p.errorf("unexpected %s", p.describe())
			p.recover()
		}
		p.endOfStatement()
		p.skipNewlines()
	}
	p.expect(token.RBrace)
	p.noRecordLit = saved
	return p.tree.addList(Block, tok, stmts)
}

func (p *parser) stmt() NodeID {
	for p.at(token.DocComment) {
		p.advance()
		p.skipNewlines()
	}
	switch {
	case p.at(token.KwLet):
		return p.letStmt()
	case p.at(token.KwDefer):
		return p.deferStmt()
	case p.atIdent("errdefer") && p.errdeferAhead():
		return p.oldErrdefer()
	case p.atIdent("var") && p.peek(1) == token.Ident:
		return p.varStmt()
	case p.atIdent("while") && p.whileAhead():
		return p.whileStmt()
	case p.atIdent("elif") && p.lineHasBrace():
		return p.elifStmt()
	case p.at(token.KwFn) && p.peek(1) == token.Ident:
		return p.nestedFn()
	case p.atIdent("static") && p.staticDeclAhead():
		return p.nestedStatic()
	}
	e := p.expr()
	if e == 0 {
		return 0
	}
	if p.at(token.Assign) || p.tok().Kind.IsCompoundAssign() {
		op := p.advance()
		p.skipNewlinesAfterAssign()
		return p.node2(AssignStmt, op, e, p.expr())
	}
	return p.node1(ExprStmt, p.tree.nodes[e].Tok, e)
}

func (p *parser) letStmt() NodeID {
	tok := p.expect(token.KwLet)
	var flags uint32
	if p.eat(token.KwMut) {
		flags |= FlagMut
	}
	return p.letBinding(tok, flags)
}

func (p *parser) letBinding(tok, flags uint32) NodeID {
	pat := p.pattern()
	var typ NodeID
	if p.eat(token.Colon) {
		typ = p.typ()
	}
	p.expect(token.Assign)
	p.skipNewlinesAfterAssign()
	init := p.expr()
	var elseBlock NodeID
	if p.at(token.KwElse) {
		p.advance()
		elseBlock = p.block()
	}
	return p.tree.addRec(LetStmt, tok, []uint32{flags, uint32(pat), uint32(typ), uint32(init), uint32(elseBlock)})
}

// condition parses an expression where a bare record literal is not allowed.
func (p *parser) condition() NodeID {
	saved := p.noRecordLit
	p.noRecordLit = true
	e := p.expr()
	p.noRecordLit = saved
	return e
}

func (p *parser) matchExpr() NodeID {
	tok := p.expect(token.KwMatch)
	scrutinee := p.condition()
	var arms []NodeID
	p.layoutList(token.LBrace, token.RBrace, func() {
		pat := p.pattern()
		var guard NodeID
		if p.eat(token.KwIf) {
			guard = p.condition()
		}
		p.expect(token.Arrow)
		p.skipNewlines()
		body := p.expr()
		p.armAssignment()
		arms = append(arms, p.tree.addRec(MatchArm, p.tree.nodes[pat].Tok, []uint32{uint32(pat), uint32(guard), uint32(body)}))
	})
	return p.node2(MatchExpr, tok, scrutinee, p.tree.addList(List, tok, arms))
}

// forExpr parses the three loop forms. A top-level `in` before the body brace
// selects the iteration form.
func (p *parser) forExpr() NodeID {
	tok := p.expect(token.KwFor)
	if p.at(token.LBrace) {
		return p.tree.addRec(ForExpr, tok, []uint32{0, 0, uint32(p.block())})
	}
	if p.cStyleForAhead() {
		return p.cStyleFor(tok)
	}
	if p.hasTopLevelIn() {
		pat := p.pattern()
		p.expect(token.KwIn)
		iter := p.condition()
		return p.tree.addRec(ForExpr, tok, []uint32{uint32(pat), uint32(iter), uint32(p.block())})
	}
	cond := p.condition()
	return p.tree.addRec(ForExpr, tok, []uint32{0, uint32(cond), uint32(p.block())})
}

func (p *parser) hasTopLevelIn() bool {
	depth := 0
	for i := p.pos; int(i) < len(p.toks); i++ {
		switch p.toks[i].Kind {
		case token.LParen, token.LBracket, token.LBrace:
			depth++
		case token.RParen, token.RBracket, token.RBrace:
			depth--
		case token.KwIn:
			if depth == 0 {
				return true
			}
		case token.Newline:
			// A newline inside a pattern's `{...}` does not end the for header.
			if depth <= 0 {
				return false
			}
		case token.EOF:
			return false
		}
		if depth < 0 {
			return false
		}
	}
	return false
}
