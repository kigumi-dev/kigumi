package syntax

import "kigumi/internal/lang/token"

// escapableType parses a parameter type, where `escaping` is a contextual keyword before a type.
// It reports whether `escaping` was written before a non-fn type; before a fn type it sets the fn type's flag.
func (p *parser) escapableType() (NodeID, bool) {
	if !p.enterDepth() {
		return 0, false
	}
	defer p.exitDepth()
	if !p.atIdent("escaping") {
		return p.typPostfix(p.typPrimary()), false
	}
	switch next := p.peek(1); {
	case isFnTypeStart(next):
		p.advance()
		return p.typPostfix(p.fnType(true)), false
	case isTypeStart(next):
		p.advance()
		return p.typPostfix(p.typPrimary()), true
	}
	return p.typPostfix(p.typPrimary()), false
}

func isTypeStart(k token.Kind) bool {
	switch k {
	case token.Ident, token.Star, token.Amp, token.LParen:
		return true
	}
	return false
}

func isFnTypeStart(k token.Kind) bool {
	switch k {
	case token.KwPure, token.KwNoalloc, token.KwUnsafe, token.KwFn, token.KwAsync, token.KwExtern:
		return true
	}
	return false
}

// fnType parses `pure noalloc unsafe fn(A, B) -> R`, `pure? fn(A) -> R`
// (effect polymorphism) and `extern(C) fn(A, ...) -> R`.
func (p *parser) fnType(escaping bool) NodeID {
	start := p.pos
	var mods uint32
	var abi uint32
	for {
		switch {
		case p.at(token.KwPure):
			tok := p.tok()
			p.advance()
			bit := ModPure
			if p.at(token.Question) && !p.tok().HasSpaceBefore() {
				p.advance()
				bit = ModPureVar
			}
			if mods&(ModPure|ModPureVar) != 0 {
				p.warnRedundantModifier(tok.Span)
			}
			mods |= bit
			continue
		case p.at(token.KwNoalloc), p.at(token.KwUnsafe):
			bit := modBit(p.tok().Kind)
			if mods&bit != 0 {
				p.warnRedundantModifier(p.tok().Span)
			}
			mods |= bit
			p.advance()
			continue
		case p.at(token.KwAsync):
			p.errorf("`async fn(...)` is not a first-class function type; call async declarations directly")
			p.advance()
			continue
		case p.at(token.KwExtern):
			p.advance()
			p.expect(token.LParen)
			abi = p.expect(token.Ident)
			p.expect(token.RParen)
			continue
		}
		break
	}
	p.expect(token.KwFn)
	var params []NodeID
	var flags uint32
	if escaping {
		flags |= FlagEscaping
	}
	p.layoutList(token.LParen, token.RParen, func() {
		if flags&FlagVariadic != 0 {
			p.errorf("only the last parameter may be variadic")
		}
		if p.at(token.Ellipsis) {
			flags |= FlagVariadic
			p.advance()
			if p.at(token.RParen) || p.at(token.Comma) {
				if abi == 0 {
					p.errorf("`...` needs an element type outside `extern(C)`: write `fn(...T) -> R`")
				}
				return
			}
			if abi != 0 {
				p.errorf("an `extern(C)` function type takes a bare `...`")
			}
		}
		typ, esc := p.escapableType()
		if esc {
			if len(params) >= MaxEscapingParams {
				p.errorf("`escaping` is supported on at most %d parameters of a fn type", MaxEscapingParams)
			} else {
				flags |= FlagEscapingParam0 << len(params)
			}
		}
		params = append(params, typ)
	})
	p.expect(token.Arrow)
	ret := p.typ()
	return p.tree.addRec(TypeFn, start, []uint32{mods, abi, uint32(p.tree.addList(List, start, params)), flags, uint32(ret)})
}
