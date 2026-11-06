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
	for i := syntax.FirstDocTok(p.t, doc.Index()); i <= doc.Index(); i++ {
		switch p.t.Toks[i].Kind {
		case token.DocComment, token.Comment, token.BlockComment:
			p.write(p.tok(i))
			p.newline()
		}
	}
	p.skipTo(doc.Index() + 1)
}

func (p *printer) attrs(list syntax.ListView) {
	var impls []syntax.NodeID
	for a := range list.Views() {
		if a.Kind() == syntax.UnsafeGuarantee {
			p.unsafeGuarantee(a.ID())
			p.attrLineEnd(a.ID())
			continue
		}
		attr := a.AsAttr()
		if attr.Path().Join(".") == "impl" {
			impls = attr.Args().AppendIDs(impls)
			continue
		}
		p.attr(attr)
		p.attrLineEnd(a.ID())
	}
	if len(impls) > 0 {
		p.write("@impl(")
		for i, e := range impls {
			if i > 0 {
				p.write(", ")
			}
			p.expr(e)
		}
		p.write(")")
		p.newline()
	}
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
	if args := a.Args(); args.Valid() {
		p.write("(")
		p.exprList(args.ID())
		p.write(")")
	}
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
		p.write(" -> ")
		p.typ(ret.ID())
	}
	if stray {
		p.write(" !")
	}
	if body := f.Body(); body.Valid() {
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
