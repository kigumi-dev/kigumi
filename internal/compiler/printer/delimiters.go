package printer

import "kigumi/internal/lang/token"

func (p *printer) indexDelimiters() {
	p.closing = make([]uint32, len(p.t.Toks))
	var stack []uint32
	for i, tk := range p.t.Toks[:p.eof()] {
		switch tk.Kind {
		case token.LParen, token.LBracket, token.LBrace:
			stack = append(stack, uint32(i))
		case token.RParen, token.RBracket, token.RBrace:
			if len(stack) > 0 {
				p.closing[stack[len(stack)-1]] = uint32(i)
				stack = stack[:len(stack)-1]
			}
		}
	}
}
