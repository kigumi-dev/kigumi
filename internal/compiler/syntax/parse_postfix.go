package syntax

import "kigumi/internal/lang/token"

// postfix parses a primary followed by calls, indexing, member access, `?`
// and a record literal, then applies the whitespace-call rule.
func (p *parser) postfix() NodeID {
	e := p.postfixChain(p.primary())
	if e == 0 {
		return 0
	}
	if !p.isWsCallee(e) {
		return e
	}
	if !p.startsWsArg() {
		p.wsCallHint(false)
		return e
	}
	argTok := p.pos
	args := []NodeID{p.postfixChain(p.primary())}
	for p.startsWsArg() {
		args = append(args, p.postfixChain(p.primary()))
	}
	p.wsCallHint(true)
	p.printOperand(e)
	return p.node2(WsCallExpr, p.tree.nodes[e].Tok, e, p.tree.addList(List, argTok, args))
}

// wsCallHint names the parenthesized form for what a whitespace call cannot spell, consuming the operand to keep recovery local.
func (p *parser) wsCallHint(hasArgs bool) {
	switch {
	case p.at(token.Ellipsis) && p.tok().HasSpaceBefore():
		p.errorf("spread requires a parenthesized call: `f(...values)`")
	case hasArgs && p.at(token.Colon):
		p.errorf("named arguments require a parenthesized call: `f(name: value)`")
	case p.at(token.Ident) && p.tok().HasSpaceBefore() && p.text() == "_":
		p.errorf("pipeline placeholders require a parenthesized call: `value |> f(a, _)`")
		p.advance()
		return
	default:
		return
	}
	p.advance()
	p.postfixChain(p.primary())
}

func (p *parser) postfixChain(e NodeID) NodeID {
	for e != 0 {
		switch {
		case p.at(token.LParen):
			open := p.pos
			args := p.exprList()
			e = p.node2(CallExpr, p.tree.nodes[e].Tok, e, p.tree.addList(List, open, args))
		case p.at(token.LBracket) && !p.tok().HasSpaceBefore():
			var items []NodeID
			tok := p.layoutList(token.LBracket, token.RBracket, func() {
				if p.startsTypeOnly() {
					items = append(items, p.typ())
				} else {
					items = append(items, p.expr())
				}
			})
			e = p.node2(BracketExpr, tok, e, p.tree.addList(List, tok, items))
		case p.at(token.Dot):
			p.advance()
			e = p.node1(MemberExpr, p.memberName(), e)
		case p.at(token.QuestionDot):
			p.advance()
			e = p.node1(OptMemberExpr, p.memberName(), e)
		case p.at(token.Question) && !p.tok().HasSpaceBefore():
			e = p.node1(TryExpr, p.advance(), e)
		case p.at(token.Newline) && p.peekPastNewlines() == token.Dot:
			p.skipNewlines()
		case p.at(token.LBrace) && p.isRecordHead(e) && (!p.noRecordLit || p.looksLikeRecordBody()):
			e = p.recordLit(e)
		default:
			return e
		}
	}
	return e
}

// startsTypeOnly is true at a token that opens a type but never an
// expression, so a bracket list item there is a type argument.
func (p *parser) startsTypeOnly() bool {
	return p.at(token.KwExtern) || p.at(token.KwFn) || p.at(token.KwPure) || p.at(token.KwNoalloc) || p.at(token.Star)
}

func (p *parser) memberName() uint32 {
	if p.at(token.Ident) {
		return p.advance()
	}
	p.errorf("expected member name, found %s", p.describe())
	return p.pos
}

// isRecordHead reports whether e has the shape of a type name: an identifier
// or member path, optionally with `[T]` arguments. Case is not consulted;
// whether the name is a type is decided by name resolution.
func (p *parser) isRecordHead(e NodeID) bool {
	n := p.tree.nodes[e]
	switch n.Kind {
	case Ident, MemberExpr:
		return true
	case BracketExpr:
		return p.isRecordHead(NodeID(n.Lhs))
	}
	return false
}

// looksLikeRecordBody tells a record literal from a block in a condition:
// a body opening with `name:` or `..` cannot start a block.
func (p *parser) looksLikeRecordBody() bool {
	i := 1
	for p.peek(i) == token.Newline {
		i++
	}
	return p.peek(i) == token.DotDot || p.peek(i) == token.Ident && p.peek(i+1) == token.Colon
}

func (p *parser) recordLit(head NodeID) NodeID {
	var entries []NodeID
	tok := p.layoutList(token.LBrace, token.RBrace, func() {
		if p.at(token.DotDot) {
			dots := p.advance()
			entries = append(entries, p.node1(Spread, dots, p.expr()))
			return
		}
		name := p.expect(token.Ident)
		p.expect(token.Colon)
		p.skipNewlines()
		entries = append(entries, p.node1(FieldInit, name, p.expr()))
	})
	return p.node2(RecordLit, tok, head, p.tree.addList(List, tok, entries))
}

func (p *parser) isWsCallee(e NodeID) bool {
	switch p.tree.nodes[e].Kind {
	case Ident, MemberExpr:
		return true
	}
	return false
}

// startsWsArg reports whether the current token begins a primary expression
// on the same line, separated by whitespace from the callee.
func (p *parser) startsWsArg() bool {
	t := p.tok()
	if !t.HasSpaceBefore() {
		return false
	}
	switch t.Kind {
	case token.Ident:
		return p.text() != "_"
	case token.Int, token.Float, token.String, token.ByteString, token.Char, token.KwTrue, token.KwFalse,
		token.Dollar:
		return true
	}
	return false
}
