package syntax

import (
	"kigumi/internal/diag"
	"kigumi/internal/token"
)

// deferStmt parses `defer body` and the failure-only `defer(fail) body`.
func (p *parser) deferStmt() NodeID {
	kw := p.advance()
	if p.at(token.LParen) && p.peek(1) == token.KwFail && p.peek(2) == token.RParen {
		p.advance()
		p.advance()
		p.advance()
		return p.node1(DeferFailStmt, kw, p.cleanupBody())
	}
	return p.node1(DeferStmt, kw, p.cleanupBody())
}

// errdeferAhead tells the removed `errdefer` statement apart from an identifier of that name.
func (p *parser) errdeferAhead() bool {
	next := p.peek(1)
	return next == token.LBrace || next == token.Ident || next.IsKeyword()
}

func (p *parser) oldErrdefer() NodeID {
	kw := p.advance()
	p.lastErr = kw
	p.bag.Add(diag.Errorf(diag.Location{Span: p.toks[kw].Span}, "`errdefer` is no longer a keyword").
		WithHelp("write `defer(fail)` for a cleanup that runs only when the scope exits with an error"))
	return p.node1(DeferFailStmt, kw, p.cleanupBody())
}

// cleanupBody is the rest of the line as one statement, or a block.
func (p *parser) cleanupBody() NodeID {
	if p.at(token.LBrace) {
		return p.block()
	}
	return p.stmt()
}
