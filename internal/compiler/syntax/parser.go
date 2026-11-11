package syntax

import (
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

// Parse always returns a tree. Errors are recorded in Tree.Diags and the
// parser resynchronizes at statement boundaries so later code is still parsed.
func Parse(f *token.File) *Tree {
	var bag diag.Bag
	bag.Limit = 100
	toks := Lex(f.Src, &bag)
	p := &parser{toks: toks, tree: newTree(f, toks), bag: &bag, pos: 1, closer: matchClosers(toks)}
	p.tree.Root = p.parseFile()
	p.tree.interp = p.interp
	p.tree.Diags = bag.Sorted()
	p.tree.DiagsDropped = bag.Dropped()
	for i := range p.tree.Diags {
		p.tree.Diags[i].Loc.Source = f.ID
		for j := range p.tree.Diags[i].Notes {
			p.tree.Diags[i].Notes[j].Loc.Source = f.ID
		}
		for j := range p.tree.Diags[i].Fixes {
			p.tree.Diags[i].Fixes[j].Loc.Source = f.ID
		}
	}
	return p.tree
}

type parser struct {
	toks []token.Token
	pos  uint32
	tree *Tree
	bag  *diag.Bag
	// noRecordLit disables `Name { ... }` record literals in condition,
	// iterable and scrutinee positions.
	noRecordLit bool
	lastErr     uint32
	splitAssign bool
	depth       uint32
	// closer[i] is the matching close-bracket index for an open bracket at i, precomputed once so lambda-vs-paren disambiguation is O(1).
	closer []uint32
	// interp caches interpolation-range discovery for string literals, keyed
	// by opening-quote offset (see interpolationRanges).
	interp map[int]interpInfo
}

func (p *parser) at(k token.Kind) bool { return p.toks[p.pos].Kind == k }

func (p *parser) tok() token.Token { return p.toks[p.pos] }

func (p *parser) text() string { return p.toks[p.pos].Text(p.tree.File.Src) }

func (p *parser) atIdent(name string) bool {
	return p.at(token.Ident) && p.text() == name
}

func (p *parser) advance() uint32 {
	i := p.pos
	if !p.at(token.EOF) {
		p.pos++
	}
	p.skipComments()
	return i
}

// skipComments skips comments that are not doc comments. Doc comments stay
// visible so declarations can pick them up.
func (p *parser) skipComments() {
	for p.at(token.Comment) || p.at(token.BlockComment) {
		p.pos++
	}
}

func (p *parser) skipNewlines() {
	for p.at(token.Newline) || p.at(token.Comment) || p.at(token.BlockComment) {
		p.pos++
	}
}

// A declaration keyword on the next line ends an unfinished initializer instead of continuing it, so recovery stays local.
func (p *parser) skipNewlinesAfterAssign() {
	if p.at(token.Newline) && startsDecl(p.peekPastNewlines()) {
		return
	}
	p.skipNewlines()
}

func (p *parser) eat(k token.Kind) bool {
	if p.at(k) {
		p.advance()
		return true
	}
	return false
}

func (p *parser) eatIdent(name string) bool {
	if p.atIdent(name) {
		p.advance()
		return true
	}
	return false
}

func (p *parser) expect(k token.Kind) uint32 {
	if k == token.Assign && p.splitAssign {
		p.splitAssign = false
		return p.pos - 1
	}
	if p.at(k) {
		return p.advance()
	}
	d := syntaxError(p.tok().Span, sprintf("expected %s, found %s", k, p.describe()))
	if open, ok := p.unclosedOpener(k); ok {
		d = d.WithNote(diag.Location{Span: p.toks[open].Span}, "`"+p.toks[open].Kind.String()+"` is opened here")
	}
	p.report(d)
	return p.pos
}

func (p *parser) describe() string {
	t := p.tok()
	switch t.Kind {
	case token.EOF:
		return "end of file"
	case token.Newline:
		return "end of line"
	case token.Ident, token.Operator, token.Int, token.Float:
		return "`" + t.Text(p.tree.File.Src) + "`"
	}
	return "`" + t.Kind.String() + "`"
}

func (p *parser) errorf(format string, args ...any) {
	p.errorAt(p.tok().Span, format, args...)
}

func (p *parser) errorAt(span token.Span, format string, args ...any) {
	p.errorHelp(span, "", format, args...)
}

// errorHelp adds a help line to the error; one error per token keeps cascades quiet.
func (p *parser) errorHelp(span token.Span, help, format string, args ...any) {
	d := syntaxError(span, sprintf(format, args...))
	if help != "" {
		d = d.WithHelp(help)
	}
	p.report(d)
}

func (p *parser) report(d diag.Diagnostic) {
	if p.pos == p.lastErr && p.bag.Len() > 0 {
		return
	}
	p.lastErr = p.pos
	p.bag.Add(d)
}

// warnRedundantModifier reports a repeated modifier and attaches the safe fix
// that deletes the second spelling together with the whitespace before it.
func (p *parser) warnRedundantModifier(span token.Span) {
	p.warnRedundant(span, "duplicate modifier")
}

// warnRedundant reports a repeated prefix element; the fix removes the second
// spelling together with the whitespace before it.
func (p *parser) warnRedundant(span token.Span, msg string) {
	src := p.tree.File.Src
	start, end := int(span.Start), int(span.End)
	// `pure?` duplicates warn on the keyword alone; take the marker too.
	if end < len(src) && src[end] == '?' {
		end++
	}
	for start > 0 && (src[start-1] == ' ' || src[start-1] == '\t') {
		start--
	}
	fix := diag.Fix{
		Title: "Remove duplicate modifier",
		Loc:   diag.Location{Span: token.Span{Start: token.Pos(start), End: token.Pos(end)}},
	}
	d := diag.Warnf(diag.Location{Span: span}, msg)
	d.Code = RedundantModifierCode
	d.Fixes = []diag.Fix{fix}
	p.bag.Add(d)
}

// visibilitySpan widens a visibility node to its closing parenthesis: the
// Span of a qualifierless `pub(module)` would otherwise stop after `pub`,
// and a fix built from it would leave `(module)` behind.
func (p *parser) visibilitySpan(id NodeID) token.Span {
	sp := p.tree.Span(id)
	if p.tree.nodes[id].Lhs == 0 {
		return sp
	}
	src := p.tree.File.Src
	end := int(sp.End)
	for end < len(src) && src[end] != ')' && src[end] != '\n' {
		end++
	}
	if end < len(src) && src[end] == ')' {
		end++
	}
	return token.Span{Start: sp.Start, End: token.Pos(end)}
}

// sameVisibility reports whether two visibility nodes name the same
// visibility, so repeating it is redundant rather than conflicting. The
// qualifier lives in Lhs and the `pub(in path)` path in Rhs.
func (p *parser) sameVisibility(a, b NodeID) bool {
	na, nb := p.tree.nodes[a], p.tree.nodes[b]
	if (na.Lhs == 0) != (nb.Lhs == 0) {
		return false
	}
	if na.Lhs != 0 {
		src := p.tree.File.Src
		if p.tree.Toks[na.Lhs].Text(src) != p.tree.Toks[nb.Lhs].Text(src) {
			return false
		}
	}
	if (na.Rhs == 0) != (nb.Rhs == 0) {
		return false
	}
	return na.Rhs == 0 || p.sameText(NodeID(na.Rhs), NodeID(nb.Rhs))
}

// sameText reports whether two nodes spell the same token sequence.
func (p *parser) sameText(a, b NodeID) bool {
	as, bs := p.tree.Span(a), p.tree.Span(b)
	src := p.tree.File.Src
	return as.End <= token.Pos(len(src)) && bs.End <= token.Pos(len(src)) && string(src[as.Start:as.End]) == string(src[bs.Start:bs.End])
}

func (p *parser) spanFrom(start uint32) token.Span {
	end := p.pos
	if end > start {
		end--
	}
	return token.Span{Start: p.toks[start].Start, End: p.toks[end].End}
}

// recover skips to the next statement boundary (newline at brace depth zero, unmatched }, or EOF); parens are ignored so one missing ) can't swallow the rest of the file.
func (p *parser) recover() {
	depth := 0
	for !p.at(token.EOF) {
		switch p.tok().Kind {
		case token.LBrace:
			depth++
		case token.RBrace:
			if depth == 0 {
				return
			}
			depth--
		case token.Newline:
			if depth == 0 {
				return
			}
		}
		p.pos++
	}
}

// endOfStatement accepts a newline, `;`, `}` (not consumed) or EOF.
func (p *parser) endOfStatement() {
	switch {
	case p.at(token.Newline), p.at(token.Semicolon):
		p.advance()
	case p.at(token.RBrace), p.at(token.EOF):
	case p.at(token.DocComment):
		p.advance()
	default:
		p.errorf("expected end of statement, found %s", p.describe())
		p.recover()
	}
}
