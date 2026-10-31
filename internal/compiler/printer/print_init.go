package printer

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// initExpr prints ` op` and the expression after it; comments written between the two stay in front of the expression.
func (p *printer) initExpr(op string, id syntax.NodeID) {
	p.write(" " + op)
	toks := p.t.Toks
	start := p.t.Span(id).Start
	first := p.cm.next
	for first < len(toks) && toks[first].Start < start {
		first++
	}
	run := first
	for run > p.cm.next && (toks[run-1].Kind.IsTrivia() || toks[run-1].Kind == token.Newline) {
		run--
	}
	var comments []token.Token
	for _, tk := range toks[run:first] {
		if tk.Kind.IsTrivia() {
			comments = append(comments, tk)
		}
	}
	if len(comments) == 0 || p.pendingBefore(run) {
		p.write(" ")
		p.expr(id)
		return
	}
	p.cm.next = first
	p.withIndent(func() {
		lines := false
		for _, tk := range comments {
			lines = lines || tk.Kind != token.BlockComment
		}
		if lines {
			p.indent++
			p.newline()
		} else {
			p.write(" ")
		}
		for _, tk := range comments {
			p.write(tk.Text(p.t.File.Src))
			if lines {
				p.newline()
			} else {
				p.write(" ")
			}
		}
		p.expr(id)
	})
}

// pendingBefore reports an unprinted comment in front of token index limit.
func (p *printer) pendingBefore(limit int) bool {
	for _, tk := range p.t.Toks[p.cm.next:limit] {
		if tk.Kind.IsTrivia() {
			return true
		}
	}
	return false
}
