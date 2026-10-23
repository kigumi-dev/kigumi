package printer

import "kigumi/internal/token"

// sameLineComment finds the line comment that follows token last on its line, past a comma the printer writes itself when skipComma is set.
func (p *printer) sameLineComment(last uint32, skipComma bool) (uint32, bool) {
	toks := p.t.Toks
	j := int(last) + 1
	if skipComma && j < len(toks) && toks[j].Kind == token.Comma {
		j++
	}
	if j >= len(toks) || j < p.cm.next {
		return 0, false
	}
	switch toks[j].Kind {
	case token.Comment:
		return uint32(j), true
	case token.BlockComment:
		if j+1 < len(toks) && (toks[j+1].Kind == token.Newline || toks[j+1].Kind == token.EOF) {
			return uint32(j), true
		}
	}
	return 0, false
}

// trailing writes the comment that ends the line of an element whose last token is last. Comments still pending before it stay where they are, so none is reordered.
func (p *printer) trailing(last uint32, skipComma bool) bool {
	j, ok := p.sameLineComment(last, skipComma)
	if !ok || p.hasCommentBefore(j) {
		return false
	}
	p.write(" " + p.tok(j))
	p.cm.next = int(j) + 1
	return true
}

// closeOfNext is the first `}` at or after token index from.
func (p *printer) closeOfNext(from uint32) uint32 {
	for i := int(from); i < len(p.t.Toks); i++ {
		switch p.t.Toks[i].Kind {
		case token.RBrace:
			return uint32(i)
		case token.EOF:
			return uint32(len(p.t.Toks))
		}
	}
	return uint32(len(p.t.Toks))
}

// bracePairAfter finds the first `{` at or after token index from and the `}` that closes it.
func (p *printer) bracePairAfter(from int) (open, closeIdx uint32) {
	depth := 0
	for i := from; i < len(p.t.Toks); i++ {
		switch p.t.Toks[i].Kind {
		case token.LBrace:
			if depth == 0 {
				open = uint32(i)
			}
			depth++
		case token.RBrace:
			if depth--; depth == 0 {
				return open, uint32(i)
			}
		case token.EOF:
			return open, uint32(len(p.t.Toks))
		}
	}
	return open, uint32(len(p.t.Toks))
}

// nextCode is the first token at or after i that is neither a newline nor a comment.
func (p *printer) nextCode(i uint32) uint32 {
	toks := p.t.Toks
	for int(i) < len(toks) {
		if k := toks[i].Kind; k != token.Newline && !k.IsTrivia() {
			break
		}
		i++
	}
	return i
}
