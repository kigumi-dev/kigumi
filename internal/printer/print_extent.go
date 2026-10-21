package printer

import (
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

func (p *printer) closingBrace(id syntax.NodeID) uint32 {
	start := p.t.Span(id).Start
	depth := 0
	for i, tk := range p.t.Toks {
		if tk.Kind == token.EOF {
			break
		}
		if tk.Start < start {
			continue
		}
		switch tk.Kind {
		case token.LBrace:
			depth++
		case token.RBrace:
			if depth--; depth == 0 {
				return uint32(i)
			}
		}
	}
	return uint32(len(p.t.Toks))
}

func (p *printer) emptyBlock(id syntax.NodeID) {
	closing := p.closingBrace(id)
	if !p.hasCommentBefore(closing) {
		p.write("{}")
		return
	}
	p.write("{")
	p.newline()
	p.indent++
	p.flushComments(closing)
	p.indent--
	p.write("}")
}

func (p *printer) hasCommentBefore(limit uint32) bool {
	for i := p.cm.next; i < int(limit) && i < len(p.t.Toks); i++ {
		if k := p.t.Toks[i].Kind; k == token.Comment || k == token.BlockComment {
			return true
		}
	}
	return false
}

// A node's span can stop before the closing delimiters of its last call or block.
func (p *printer) lastTok(s syntax.NodeID) uint32 {
	sp := p.t.Span(s)
	first, last := -1, uint32(0)
	for i, tk := range p.t.Toks {
		// Import path segments are synthetic tokens appended after EOF.
		if tk.Kind == token.EOF {
			break
		}
		if tk.End <= sp.End && tk.Start >= sp.Start {
			if first < 0 {
				first = i
			}
			last = uint32(i)
		}
	}
	if first < 0 {
		return last
	}
	depth := 0
	for i := first; i < len(p.t.Toks) && p.t.Toks[i].Kind != token.EOF; i++ {
		switch p.t.Toks[i].Kind {
		case token.LParen, token.LBracket, token.LBrace:
			depth++
		case token.RParen, token.RBracket, token.RBrace:
			depth--
		}
		if i >= int(last) && depth <= 0 {
			return uint32(i)
		}
	}
	return last
}
