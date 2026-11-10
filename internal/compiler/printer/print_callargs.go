package printer

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// callArgs prints `(args)`. Inline block comments stay beside their argument;
// a line comment, an own-line block comment, or a comment combined with an
// argument the printer breaks over several lines puts each argument on its own
// line so every comment stays where it belongs.
func (p *printer) callArgs(call, callee syntax.NodeID, args syntax.ListView) {
	open := p.lastTok(callee) + 1
	if int(open) >= len(p.t.Toks) || p.t.Toks[open].Kind != token.LParen {
		p.write("(")
		p.exprList(args.ID())
		p.write(")")
		return
	}
	p.exprListDelimited(args.ID(), open, p.lastTok(call), false)
}

// exprListDelimited prints a parenthesized comma-separated expression list.
// open and closing are the token indexes of the parens in the source.
// loneComma keeps the comma that makes a one-element tuple a tuple.
func (p *printer) exprListDelimited(list syntax.NodeID, open, closing uint32, loneComma bool) {
	ids := p.t.List(list).AppendIDs(nil)
	if len(ids) == 0 {
		p.write("(")
		if p.noteLineBreak(open+1, closing) {
			p.newline()
			p.indent++
			p.flushComments(closing)
			p.indent--
		} else {
			p.inlineBlocks(open+1, closing, false, false)
		}
		p.write(")")
		return
	}
	if p.listLineBreak(open, closing, ids) || p.anyCommentIn(open+1, closing) && p.listBreaksLines(ids) {
		p.verticalExprList(ids, closing)
		return
	}
	p.write("(")
	for i, id := range ids {
		if i > 0 {
			p.write(", ")
		}
		p.withIndent(func() { p.expr(id) })
	}
	if loneComma {
		p.write(",")
	}
	p.write(")")
}

// verticalExprList prints one argument per line, keeping each comment with it.
func (p *printer) verticalExprList(ids []syntax.NodeID, closing uint32) {
	p.write("(")
	p.newline()
	p.indent++
	for i, id := range ids {
		next := closing
		if i+1 < len(ids) {
			next = p.firstTok(ids[i+1])
		}
		p.flushComments(p.firstTok(id))
		p.withIndent(func() { p.expr(id) })
		p.inlineBlocks(p.lastTok(id)+1, next, true, false)
		p.write(",")
		p.trailing(p.lastTok(id), true)
		p.newline()
	}
	p.flushComments(closing)
	p.indent--
	p.write(")")
}

// listBreaksLines reports whether the printer lays out any element over
// several lines anyway, so a comment in the list cannot sit on one line.
func (p *printer) listBreaksLines(ids []syntax.NodeID) bool {
	for _, id := range ids {
		if p.multiline(id) || p.breaksLines(p.t.View(id)) {
			return true
		}
	}
	return false
}

// listLineBreak reports whether any gap in the list holds a comment that
// cannot stay inline.
func (p *printer) listLineBreak(open, closing uint32, ids []syntax.NodeID) bool {
	prev := open
	for _, id := range ids {
		if p.noteLineBreak(prev+1, p.firstTok(id)) {
			return true
		}
		prev = p.lastTok(id)
	}
	return p.noteLineBreak(prev+1, closing)
}
