package printer

import (
	"strings"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// This file holds the rules that keep comments beside the code they annotate.
// A comment that starts its own line is printed on its own line; a block
// comment written between two tokens on one line stays inline, either before
// the next element (`/* c */ x`) or after the previous one (`x /* c */`).

// lineCommentIn reports a line comment in [from, to).
func (p *printer) lineCommentIn(from, to uint32) bool {
	for i := from; i < to && int(i) < len(p.t.Toks); i++ {
		if p.t.Toks[i].Kind == token.Comment {
			return true
		}
	}
	return false
}

// inlineBlocks writes the pending block comments in the token gap [from, to)
// that started on the previous token's line, separated by single spaces.
// lead adds a space before the first one and trail after the last, so callers
// can splice them around their own separators.
func (p *printer) inlineBlocks(from, to uint32, lead, trail bool) {
	n := 0
	for i := int(max(from, uint32(p.cm.next))); i < int(to) && i < len(p.t.Toks); i++ {
		tk := p.t.Toks[i]
		if tk.Kind != token.BlockComment {
			continue
		}
		if i > 0 && p.t.Toks[i-1].Kind == token.Newline {
			break // an own-line block comment belongs on its own line
		}
		if n == 0 && lead {
			p.write(" ")
		} else if n > 0 {
			p.write(" ")
		}
		p.write(tk.Text(p.t.File.Src))
		p.cm.next = i + 1
		n++
	}
	if n > 0 && trail {
		p.write(" ")
	}
}

// noteLineBreak records whether the token gap [from, to) holds a comment the
// printer cannot keep inline: a line comment, or a block comment that starts
// a line.
func (p *printer) noteLineBreak(from, to uint32) bool {
	for i := int(max(from, uint32(p.cm.next))); i < int(to) && i < len(p.t.Toks); i++ {
		switch p.t.Toks[i].Kind {
		case token.Comment:
			return true
		case token.BlockComment:
			if i == 0 || p.t.Toks[i-1].Kind == token.Newline {
				return true
			}
		}
	}
	return false
}

// leadingInline writes the block comments that sit immediately before a node's
// first token on the same line, so `y: /* b */ 2` keeps its comment instead of
// flushing it after the whole statement.
func (p *printer) leadingInline(id syntax.NodeID) {
	first := p.firstTok(id)
	if int(first) <= p.cm.next || !p.inlineBlockIn(uint32(p.cm.next), first) {
		return
	}
	if s := p.sb.String(); s != "" && !strings.ContainsRune(" \n\t([{", rune(s[len(s)-1])) {
		p.write(" ")
	}
	p.inlineBlocks(uint32(p.cm.next), first, false, true)
}

// trailingInline writes the block comments that directly follow a node on its
// own line, so `xs /* m */ .len()` keeps its comment beside the receiver.
func (p *printer) trailingInline(id syntax.NodeID) {
	last := p.lastTok(id)
	p.inlineBlocks(last+1, p.nextCode(last+1), true, false)
}

// inlineBlockIn reports a pending block comment in [from, to) that stays on
// the line of the token before it.
func (p *printer) inlineBlockIn(from, to uint32) bool {
	for i := int(max(from, uint32(p.cm.next))); i < int(to) && i < len(p.t.Toks); i++ {
		if p.t.Toks[i].Kind != token.BlockComment {
			continue
		}
		if i > 0 && p.t.Toks[i-1].Kind == token.Newline {
			return false
		}
		return true
	}
	return false
}

// openBraceComment keeps a comment that ended an opening brace's line beside
// that brace, then starts the body on the next line.
func (p *printer) openBraceComment(open uint32) {
	if open > 0 {
		p.trailing(open, false)
	}
	p.newline()
}

// parenBefore is the `(` that opens the list whose first element starts at
// first, looking back over only trivia; 0 when the token before it is not a
// parenthesis (so a comment before an attribute cannot land on the wrong one).
func (p *printer) parenBefore(first uint32) uint32 {
	for j := int(first) - 1; j >= 0; j-- {
		if k := p.t.Toks[j].Kind; k != token.Newline && !k.IsTrivia() {
			if k == token.LParen {
				return uint32(j)
			}
			return 0
		}
	}
	return 0
}

// braceBefore is the `{` that opens the list whose first element starts at
// first, looking back over only trivia.
func (p *printer) braceBefore(first uint32) uint32 {
	for j := int(first) - 1; j >= 0; j-- {
		if k := p.t.Toks[j].Kind; k != token.Newline && !k.IsTrivia() {
			if k == token.LBrace {
				return uint32(j)
			}
			return 0
		}
	}
	return 0
}

// lparenAfter is the first `(` at or after from, used for an attribute's
// argument list whose node does not expose the parenthesis.
func (p *printer) lparenAfter(from uint32) uint32 {
	for i := int(from); i < len(p.t.Toks); i++ {
		switch p.t.Toks[i].Kind {
		case token.LParen:
			return uint32(i)
		case token.LBrace, token.RBrace, token.Newline:
			return 0
		}
	}
	return 0
}
