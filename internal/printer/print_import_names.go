package printer

import (
	"sort"

	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

// importBraces finds the `{` and `}` of a braced import list within tokens first..last.
func (p *printer) importBraces(first, last uint32) (open, closing uint32, ok bool) {
	for k := first; k <= last && int(k) < len(p.t.Toks); k++ {
		switch p.t.Toks[k].Kind {
		case token.LBrace:
			open, ok = k, true
		case token.RBrace:
			return open, k, ok
		}
	}
	return 1<<31 - 1, 0, false
}

// nameTrivia is the comments one imported name carries: lines above it and the comment ending its line.
type nameTrivia struct {
	lead  []uint32
	trail uint32
}

// importNames prints a `{a, b}` list sorted by name. Comments inside the braces put each name on its own line, travelling with the name.
func (p *printer) importNames(decl syntax.NodeID, names []syntax.NodeID) {
	open, closing, ok := p.importBraces(p.firstTok(decl), p.lastTok(decl))
	sorted := p.sortedNames(names)
	if !ok || !p.anyCommentIn(open, closing) {
		p.write("{")
		for i, nm := range sorted {
			if i > 0 {
				p.write(", ")
			}
			p.importName(nm)
		}
		p.write("}")
		return
	}
	trivia, rest := p.nameTrivia(names, open, closing)
	p.write("{")
	p.newline()
	p.indent++
	for _, nm := range sorted {
		for _, c := range trivia[nm].lead {
			p.write(p.tok(c))
			p.newline()
		}
		p.importName(nm)
		p.write(",")
		if t := trivia[nm].trail; t != 0 {
			p.write(" " + p.tok(t))
		}
		p.newline()
	}
	for _, c := range rest {
		p.write(p.tok(c))
		p.newline()
	}
	p.indent--
	p.write("}")
	p.cm.next = max(p.cm.next, int(closing)+1)
}

func (p *printer) importName(nm syntax.NodeID) {
	v := p.t.View(nm)
	if v.Kind() == syntax.ImportAlias {
		a := v.AsImportAlias()
		p.write(a.Name().Name().Text() + " as " + a.Alias().Name().Text())
		return
	}
	p.write(v.Tok().Text())
}

func (p *printer) anyCommentIn(from, to uint32) bool {
	for i := from; i < to && int(i) < len(p.t.Toks); i++ {
		if k := p.t.Toks[i].Kind; k == token.Comment || k == token.BlockComment {
			return true
		}
	}
	return false
}

// nameTrivia splits the comments inside the braces among the names, in source order; rest are those after the last name.
func (p *printer) nameTrivia(names []syntax.NodeID, open, closing uint32) (map[syntax.NodeID]*nameTrivia, []uint32) {
	out := map[syntax.NodeID]*nameTrivia{}
	cursor := open + 1
	inOrder := append([]syntax.NodeID{}, names...)
	sort.Slice(inOrder, func(i, j int) bool { return p.firstTok(inOrder[i]) < p.firstTok(inOrder[j]) })
	for _, nm := range inOrder {
		t := &nameTrivia{}
		for i := cursor; i < p.firstTok(nm); i++ {
			if k := p.t.Toks[i].Kind; k == token.Comment || k == token.BlockComment {
				t.lead = append(t.lead, i)
			}
		}
		last := p.lastTok(nm)
		cursor = last + 1
		if cursor+1 < closing && p.t.Toks[cursor].Kind == token.BlockComment && p.t.Toks[cursor+1].Kind == token.Comma {
			t.trail = cursor
			cursor += 2
		} else if cursor < closing && p.t.Toks[cursor].Kind == token.Comma {
			cursor++
		}
		if t.trail == 0 && cursor < closing {
			if k := p.t.Toks[cursor].Kind; k == token.Comment || k == token.BlockComment && p.t.Toks[cursor+1].Kind == token.Newline {
				t.trail = cursor
				cursor++
			}
		}
		out[nm] = t
	}
	var rest []uint32
	for i := cursor; i < closing; i++ {
		if k := p.t.Toks[i].Kind; k == token.Comment || k == token.BlockComment {
			rest = append(rest, i)
		}
	}
	return out, rest
}
