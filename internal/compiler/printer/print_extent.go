package printer

import (
	"sort"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
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
	toks := p.t.Toks[:p.eof()]
	first := sort.Search(len(toks), func(i int) bool { return toks[i].Start >= sp.Start })
	end := sort.Search(len(toks), func(i int) bool { return toks[i].End > sp.End })
	if first >= end {
		return 0
	}
	last := uint32(end - 1)
	// A defined type's path can swallow the newline and comment that follow it.
	for last > uint32(first) && (toks[last].Kind == token.Newline || toks[last].Kind.IsTrivia()) {
		last--
	}
	depth := 0
	for i := first; i < len(toks); i++ {
		switch toks[i].Kind {
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

// firstTok is the first non-trivia token of a node, counting the doc
// comment lines that the node's doc slot points at.
func (p *printer) firstTok(id syntax.NodeID) uint32 {
	if doc := p.t.View(id).AsDecl().Doc(); doc.Valid() {
		return syntax.FirstDocTok(p.t, doc.Index())
	}
	sp := p.t.Span(id)
	toks := p.t.Toks[:p.eof()+1]
	i := sort.Search(len(toks), func(i int) bool { return toks[i].Start >= sp.Start })
	for i < len(toks) && (toks[i].Kind == token.Newline || toks[i].Kind == token.BOF) {
		i++
	}
	if i >= len(toks) {
		return uint32(len(p.t.Toks))
	}
	return uint32(i)
}

// eof is the index of the EOF token; import path segments are synthetic tokens appended after it.
func (p *printer) eof() int {
	if p.eofIdx == 0 {
		p.eofIdx = len(p.t.Toks) - 1
		for i, tk := range p.t.Toks {
			if tk.Kind == token.EOF {
				p.eofIdx = i
				break
			}
		}
	}
	return p.eofIdx
}
