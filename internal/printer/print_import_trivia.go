package printer

import (
	"strings"

	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

// importTrivia is what travels with one import when the run is sorted: the comment lines above it and the comment ending its line.
type importTrivia struct {
	lead  []uint32
	trail uint32
	// end is the index of the last token the import owns, trailing comment included.
	end uint32
}

// importRun holds the comments of a sorted import run, keyed by import.
type importRun struct {
	byDecl map[syntax.NodeID]*importTrivia
	// first is the position in the declaration list where the run starts.
	first int
	// header ends the comments before the run that keep their place at its top.
	header uint32
	// headerBlank keeps the blank line between that header and the first import.
	headerBlank bool
}

// collectImportTrivia claims the comments around each import of decls[start:end]; ok is false when one cannot travel.
func (p *printer) collectImportTrivia(decls []syntax.NodeID, start, end int) (*importRun, bool) {
	run := &importRun{byDecl: map[syntax.NodeID]*importTrivia{}}
	gapStart := uint32(0)
	if start > 0 {
		gapStart = p.lastTok(decls[start-1]) + 1
		if j, ok := p.sameLineComment(gapStart-1, false); ok {
			gapStart = j + 1
		}
	}
	for i := start; i < end; i++ {
		d := decls[i]
		first, last := p.firstTok(d), p.lastTok(d)
		open, closing, _ := p.importBraces(first, last)
		for k := first; k <= last && int(k) < len(p.t.Toks); k++ {
			if kind := p.t.Toks[k].Kind; (kind == token.Comment || kind == token.BlockComment || kind == token.DocComment) && (k < open || k > closing) {
				return nil, false
			}
		}
		tv := &importTrivia{end: last}
		if j, ok := p.sameLineComment(last, false); ok {
			tv.trail, tv.end = j, j
		}
		lead, header, ok := p.leadComments(gapStart, first, i == start)
		if !ok {
			return nil, false
		}
		tv.lead = lead
		if i == start {
			run.header = header
			run.headerBlank = p.commentBefore(gapStart, header) && header > gapStart && p.blankBefore(int(header))
		}
		run.byDecl[d] = tv
		gapStart = tv.end + 1
	}
	return run, true
}

// leadComments lists the comments in [from, to). For the first import the ones before the last blank line stay behind as a header ending at the returned index.
func (p *printer) leadComments(from, to uint32, isFirst bool) (lead []uint32, header uint32, ok bool) {
	header = from
	for i := from; i < to && int(i) < len(p.t.Toks); i++ {
		tk := p.t.Toks[i]
		switch {
		case tk.Kind == token.DocComment:
			return nil, 0, false
		case tk.Kind == token.Newline && tk.Len() > 1 && isFirst:
			lead, header = nil, i+1
		case tk.Kind == token.Comment || tk.Kind == token.BlockComment:
			if tk.Start == 0 && strings.HasPrefix(tk.Text(p.t.File.Src), "#!") {
				header = i + 1
				continue
			}
			lead = append(lead, i)
		}
	}
	if isFirst && len(lead) > 0 {
		header = lead[0]
	}
	return lead, header, true
}

func (p *printer) commentBefore(from, to uint32) bool {
	for i := from; i < to && int(i) < len(p.t.Toks); i++ {
		if k := p.t.Toks[i].Kind; k == token.Comment || k == token.BlockComment {
			return true
		}
	}
	return false
}

// importComments writes the lead comments of a carried import, one per line.
func (p *printer) importComments(tv *importTrivia) {
	for _, i := range tv.lead {
		p.write(p.tok(i))
		p.newline()
	}
}

// importLineEnd ends a carried import's line with its own trailing comment.
func (p *printer) importLineEnd(tv *importTrivia) {
	if tv.trail != 0 {
		p.write(" " + p.tok(tv.trail))
	}
	p.newline()
	p.cm.next = max(p.cm.next, int(tv.end)+1)
}
