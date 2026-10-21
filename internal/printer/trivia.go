package printer

import (
	"strings"

	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

// comments tracks which comment tokens have been emitted. Comments are
// re-attached to the next declaration or statement in source order;
// doc comments travel with their declaration through the FnDecl/TypeDecl slots.
type comments struct {
	t    *syntax.Tree
	next int
}

func newComments(t *syntax.Tree) *comments { return &comments{t: t} }

// flushComments emits every not-yet-printed comment that appears before token
// index limit, each on its own line.
func (p *printer) flushComments(limit uint32) { p.flushTrivia(limit, false) }

func (p *printer) flushDocComments(limit uint32) { p.flushTrivia(limit, true) }

func (p *printer) flushTrivia(limit uint32, docs bool) {
	toks := p.t.Toks
	for p.cm.next < len(toks) && uint32(p.cm.next) < limit {
		tk := toks[p.cm.next]
		p.cm.next++
		if tk.Kind == token.EOF {
			p.cm.next = len(toks)
			break
		}
		if !tk.Kind.IsTrivia() || tk.Kind == token.DocComment && !docs {
			continue
		}
		if p.cm.next >= 2 && p.blankBefore(p.cm.next-1) {
			p.blank()
		}
		text := tk.Text(p.t.File.Src)
		p.write(text)
		p.newline()
		if tk.Start == 0 && strings.HasPrefix(text, "#!") {
			p.blank()
		}
	}
}

// Does nothing when another kind of comment is pending, so none is reordered.
func (p *printer) inlineBlockComments(from uint32, kind token.Kind) {
	toks := p.t.Toks
	end := int(from)
	for end < len(toks) && toks[end].Kind != kind {
		end++
	}
	var found []token.Token
	for i := p.cm.next; i < end; i++ {
		switch toks[i].Kind {
		case token.BlockComment:
			found = append(found, toks[i])
		case token.Comment, token.DocComment:
			return
		}
	}
	for _, tk := range found {
		p.write(" " + tk.Text(p.t.File.Src))
	}
	if len(found) > 0 {
		p.cm.next = end
	}
}

// skipTo marks comments before limit as consumed without printing; used for
// doc comments that the declaration prints itself.
func (p *printer) skipTo(limit uint32) {
	if uint32(p.cm.next) < limit {
		p.cm.next = int(limit)
	}
}

// blankBefore reports whether the token at i is preceded by an empty line.
func (p *printer) blankBefore(i int) bool {
	if i == 0 {
		return false
	}
	prev := p.t.Toks[i-1]
	if prev.Kind != token.Newline {
		return false
	}
	return prev.Len() > 1
}

func (p *printer) peekTrailing(afterTok uint32) string {
	i := int(afterTok) + 1
	if i < len(p.t.Toks) && (p.t.Toks[i].Kind == token.Comment || p.t.Toks[i].Kind == token.BlockComment) && i >= p.cm.next {
		return p.t.Toks[i].Text(p.t.File.Src)
	}
	return ""
}
