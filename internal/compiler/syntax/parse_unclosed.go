package syntax

import "kigumi/internal/lang/token"

func openerOf(closer token.Kind) (token.Kind, bool) {
	switch closer {
	case token.RParen:
		return token.LParen, true
	case token.RBrace:
		return token.LBrace, true
	case token.RBracket:
		return token.LBracket, true
	}
	return 0, false
}

func (p *parser) unclosedOpener(closer token.Kind) (uint32, bool) {
	want, ok := openerOf(closer)
	if !ok {
		return 0, false
	}
	var stack []uint32
	for i := uint32(0); i < p.pos && int(i) < len(p.toks); i++ {
		switch p.toks[i].Kind {
		case token.LParen, token.LBracket, token.LBrace:
			stack = append(stack, i)
		case token.RParen, token.RBracket, token.RBrace:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	for j := len(stack) - 1; j >= 0; j-- {
		if p.toks[stack[j]].Kind == want {
			return stack[j], true
		}
	}
	return 0, false
}
