package printer

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// binary prints operators with single spaces; a chain of the same leading
// operator (`|>`, `&&`, `||`) that spans lines in the source, or whose left
// side is itself the same chain, goes one operand per line with the
// operator leading.
func (p *printer) binary(id syntax.NodeID) {
	b := p.t.View(id).AsBinary()
	if isLeadingOp(b.Op().Kind()) && p.multilineChain(b) {
		p.opChain(b)
		return
	}
	p.expr(b.Left().ID())
	switch b.Op().Kind() {
	case token.DotDot, token.DotDotEq:
		p.write(b.Op().Text())
	default:
		p.write(" " + b.Op().Text())
		if c, ok := p.takeLineEndOp(b.Op().Index()); ok {
			p.write(" " + c)
			p.indent++
			p.newline()
			p.expr(b.Right().ID())
			p.indent--
			return
		}
		p.write(" ")
	}
	p.inlineBlocks(b.Op().Index()+1, p.firstTok(b.Right().ID()), false, true)
	p.expr(b.Right().ID())
}

// takeLineEndOp consumes the comment that ends the operator's line, so the
// right operand moves to the next line with the comment still beside the
// operator instead of trailing the whole statement.
func (p *printer) takeLineEndOp(op uint32) (string, bool) {
	j, ok := p.sameLineComment(op, false)
	if !ok || p.hasCommentBefore(j) {
		return "", false
	}
	p.cm.next = int(j) + 1
	return p.tok(j), true
}

// isLeadingOp reports whether k's canonical multiline form leads with the
// operator rather than trailing it.
func isLeadingOp(k token.Kind) bool {
	switch k {
	case token.PipeGt, token.AndAnd, token.OrOr:
		return true
	}
	return false
}

// multilineChain reports whether the chain rooted at id actually breaks
// across lines in the source anywhere along its same-operator links — not
// merely that it has 3+ terms, which a same-operator LHS also indicates.
func (p *printer) multilineChain(b syntax.BinaryView) bool {
	if p.opSplitsLine(b.Op().Index()) {
		return true
	}
	if left := b.Left(); left.Kind() == syntax.Binary && left.AsBinary().Op().Kind() == b.Op().Kind() {
		return p.multilineChain(left.AsBinary())
	}
	return false
}

// opSplitsLine reports whether the source actually breaks the line at the
// operator itself (leading or trailing form), rather than merely containing
// a multi-line RHS such as a match/if block or a trailing-comma call — those
// span multiple lines without the operator marking a chain split.
func (p *printer) opSplitsLine(opTok uint32) bool {
	for i := int(opTok) - 1; i >= 0; i-- {
		switch p.t.Toks[i].Kind {
		case token.Newline:
			return true
		case token.Comment, token.BlockComment, token.DocComment:
			continue
		}
		break
	}
	for i := int(opTok) + 1; i < len(p.t.Toks); i++ {
		switch p.t.Toks[i].Kind {
		case token.Newline:
			return true
		case token.Comment, token.BlockComment, token.DocComment:
			continue
		}
		break
	}
	return false
}

func (p *printer) opChain(b syntax.BinaryView) {
	left := b.Left()
	if left.Kind() == syntax.Binary && left.AsBinary().Op().Kind() == b.Op().Kind() {
		p.opChain(left.AsBinary())
	} else {
		p.expr(left.ID())
	}
	p.newline()
	p.indent++
	p.flushComments(b.Op().Index())
	p.write(b.Op().Text() + " ")
	p.indent--
	p.expr(b.Right().ID())
}
