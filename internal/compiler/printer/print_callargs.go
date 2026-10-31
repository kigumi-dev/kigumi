package printer

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// callArgs prints `(args)`. A line comment between two arguments (or before the first or after the last) moves each argument to its own line so every comment stays beside its argument.
func (p *printer) callArgs(call, callee syntax.NodeID, args syntax.ListView) {
	open, closing := p.lastTok(callee)+1, p.lastTok(call)
	if int(open) >= len(p.t.Toks) || p.t.Toks[open].Kind != token.LParen || !p.lineCommentBetweenArgs(open, closing, args) {
		p.write("(")
		p.exprList(args.ID())
		p.write(")")
		return
	}
	p.write("(")
	p.newline()
	p.indent++
	for a := range args.Views() {
		p.flushComments(p.firstTok(a.ID()))
		p.withIndent(func() { p.expr(a.ID()) })
		p.write(",")
		p.trailing(p.lastTok(a.ID()), true)
		p.newline()
	}
	p.flushComments(closing)
	p.indent--
	p.write(")")
}

func (p *printer) lineCommentBetweenArgs(open, closing uint32, args syntax.ListView) bool {
	pos := open + 1
	for a := range args.Views() {
		if p.lineCommentIn(pos, p.firstTok(a.ID())) {
			return true
		}
		pos = p.lastTok(a.ID()) + 1
	}
	return p.lineCommentIn(pos, closing)
}

func (p *printer) lineCommentIn(from, to uint32) bool {
	for i := from; i < to && int(i) < len(p.t.Toks); i++ {
		if p.t.Toks[i].Kind == token.Comment {
			return true
		}
	}
	return false
}
