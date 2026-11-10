package printer

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

func (p *printer) prefix(id syntax.NodeID) {
	d := p.t.View(id).AsDecl()
	p.docLines(d)
	if attrs := d.Attrs(); attrs.Valid() {
		p.attrs(attrs)
	}
	if vis := d.Vis(); vis.Valid() {
		p.visibility(vis)
	}
	if d.Kind() == syntax.FnDecl {
		if mods := d.AsFnDecl().Mods() &^ (syntax.ModBareErr | syntax.ModStrayBang); mods != 0 {
			p.write(modsText(mods) + " ")
		}
	}
}

// docLines prints the `///` lines of a declaration or field with any plain comments between them.
func (p *printer) docLines(d syntax.DeclView) {
	doc := d.Doc()
	if !doc.Valid() || p.cm.next > int(doc.Index()) {
		return
	}
	start := syntax.FirstDocTok(p.t, doc.Index())
	for i := start; i <= doc.Index(); i++ {
		switch p.t.Toks[i].Kind {
		case token.DocComment, token.Comment, token.BlockComment:
			p.write(p.tok(i))
			p.newline()
		}
	}
	// A comment the attribute lines own can sit before the doc block once the
	// doc moves above the attributes; print it beside the header, not skip it.
	p.flushComments(start)
	p.skipTo(doc.Index() + 1)
}

func (p *printer) attrs(list syntax.ListView) {
	views := make([]syntax.View, 0, list.Len())
	for v := range list.Views() {
		views = append(views, v)
	}
	var impls []syntax.NodeID
	// deferLineEnd holds the comments that ended impl lines printed below,
	// where they cannot stay; they move just above the merged line.
	var deferLineEnd []string
	for i, v := range views {
		if v.Kind() == syntax.UnsafeGuarantee {
			p.unsafeGuarantee(v.ID())
			p.attrLineEnd(v.ID())
			continue
		}
		attr := v.AsAttr()
		if attr.Path().Join(".") == "impl" {
			impls = append(impls, v.ID())
			if i < len(views)-1 {
				if c, ok := p.takeLineEnd(v.ID()); ok {
					deferLineEnd = append(deferLineEnd, c)
				}
			}
			continue
		}
		p.attr(attr)
		p.attrLineEnd(v.ID())
	}
	if len(impls) == 0 {
		return
	}
	// The merged `@impl(...)` line cannot hold the comments of the separate
	// lines it replaces, so they move above it; the last one keeps its line.
	last := impls[len(impls)-1]
	p.flushComments(p.firstTok(last))
	for _, c := range deferLineEnd {
		p.write(c)
		p.newline()
	}
	// A comment that ends the last impl entry's line inside the parentheses
	// stays on the merged line, after the closed argument list.
	lastEntryComment, hasLastEntryComment := p.takeArgLineEnd(last)
	p.write("@impl(")
	first := true
	for _, id := range impls {
		for _, e := range p.t.View(id).AsAttr().Args().AppendIDs(nil) {
			if !first {
				p.write(", ")
			}
			first = false
			p.expr(e)
		}
	}
	p.write(")")
	if hasLastEntryComment {
		p.write(" " + lastEntryComment)
	}
	p.attrLineEnd(last)
}

// takeArgLineEnd consumes the line comment that ends the last argument entry
// of an attribute, skipping the comma the printer writes itself.
func (p *printer) takeArgLineEnd(id syntax.NodeID) (string, bool) {
	ids := p.t.View(id).AsAttr().Args().AppendIDs(nil)
	if len(ids) == 0 {
		return "", false
	}
	j, ok := p.sameLineComment(p.lastTok(ids[len(ids)-1]), true)
	if !ok || p.hasCommentBefore(j) {
		return "", false
	}
	p.cm.next = int(j) + 1
	return p.tok(j), true
}

// takeLineEnd consumes the comment that ends the line of the attribute id and
// returns its text, so a later layout can print it in its new place.
func (p *printer) takeLineEnd(id syntax.NodeID) (string, bool) {
	j, ok := p.sameLineComment(p.lastTok(id), false)
	if !ok || p.hasCommentBefore(j) {
		return "", false
	}
	p.cm.next = int(j) + 1
	return p.tok(j), true
}

// attrLineEnd ends an attribute's line with its comment and keeps the own-line comments that follow it above the next attribute or the declaration.
func (p *printer) attrLineEnd(id syntax.NodeID) {
	last := p.lastTok(id)
	p.trailing(last, false)
	p.newline()
	p.flushComments(p.nextCode(last + 1))
}

func (p *printer) attr(a syntax.AttrView) {
	p.write("@" + a.Path().Join("."))
	args := a.Args()
	if !args.Valid() {
		return
	}
	open := p.lparenAfter(p.lastTok(a.Path().ID()))
	if open == 0 {
		p.write("(")
		p.exprList(args.ID())
		p.write(")")
		return
	}
	p.exprListDelimited(args.ID(), open, p.lastTok(a.ID()), false)
}

func (p *printer) visibility(v syntax.VisibilityView) {
	q := v.Qualifier()
	if !q.Valid() {
		p.write("pub ")
		return
	}
	switch q.Text() {
	case "self":
		return
	case "in":
		p.write("pub(in " + v.Path().Join(".") + ") ")
	default:
		p.write("pub(" + q.Text() + ") ")
	}
}

func (p *printer) fnDecl(id syntax.NodeID) {
	p.prefix(id)
	f := p.t.View(id).AsFnDecl()
	p.write("fn ")
	if recv := f.Recv(); recv.Valid() {
		p.typ(recv.ID())
		p.write(".")
	}
	p.write(f.Name().Text())
	if g := f.Generics(); g.Valid() {
		p.genericParams(g.ID())
	}
	p.params(f.Params().ID())
	stray := f.Mods()&syntax.ModStrayBang != 0
	if f.Mods()&syntax.ModBareErr != 0 {
		p.write(" !")
	} else if ret := f.Ret(); ret.Valid() && (stray || !p.isVoidType(ret.ID())) {
		p.inlineBlocks(p.lastTok(f.Params().ID())+1, p.firstTok(ret.ID()), true, false)
		p.write(" -> ")
		p.typ(ret.ID())
	}
	if stray {
		p.write(" !")
	}
	if body := f.Body(); body.Valid() {
		p.inlineBlocks(uint32(p.cm.next), p.firstTok(body.ID()), true, false)
		p.write(" ")
		p.block(body.ID())
	}
}

func (p *printer) isVoidType(id syntax.NodeID) bool {
	v := p.t.View(id)
	if v.Kind() != syntax.TypePath {
		return false
	}
	tp := v.AsTypePath()
	return !tp.Args().Valid() && tp.Path().Join(".") == "Void"
}

func (p *printer) genericParams(list syntax.NodeID) {
	p.write("[")
	for i, g := range p.t.List(list).Enumerate() {
		if i > 0 {
			p.write(", ")
		}
		p.genericParam(g.AsGenericParam())
	}
	p.write("]")
}

// genericParam prints `T`, `T: A + B`, or `const N: usize`.
func (p *printer) genericParam(g syntax.GenericParamView) {
	if g.Flags()&syntax.FlagConst != 0 {
		p.write("const ")
		p.write(g.Name().Text())
		p.write(": ")
		p.typ(g.Bound().ID())
		return
	}
	p.write(g.Name().Text())
	if bound := g.Bound(); bound.Valid() {
		p.write(": ")
		for j, b := range p.t.List(bound.ID()).Enumerate() {
			if j > 0 {
				p.write(" + ")
			}
			p.typ(b.ID())
		}
	}
}
