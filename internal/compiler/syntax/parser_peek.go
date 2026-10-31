package syntax

import "kigumi/internal/lang/token"

// peek treats plain comments as whitespace.
func (p *parser) peek(n int) token.Kind {
	i := p.pos
	for ; n > 0; n-- {
		i = p.nextSignificant(i + 1)
	}
	if int(i) >= len(p.toks) {
		return token.EOF
	}
	return p.toks[i].Kind
}

func (p *parser) nextSignificant(i uint32) uint32 {
	for int(i) < len(p.toks) && (p.toks[i].Kind == token.Comment || p.toks[i].Kind == token.BlockComment) {
		i++
	}
	return i
}

func (p *parser) peekPastNewlines() token.Kind {
	return p.peekPastNewlinesFrom(p.pos+1, false)
}

func (p *parser) peekPastNewlinesFrom(i uint32, docs bool) token.Kind {
	for ; int(i) < len(p.toks); i++ {
		switch p.toks[i].Kind {
		case token.Newline, token.Comment, token.BlockComment:
		case token.DocComment:
			if !docs {
				return token.DocComment
			}
		default:
			return p.toks[i].Kind
		}
	}
	return token.EOF
}
