package printer

import (
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

func (p *printer) prefix(id syntax.NodeID) {
	d := p.t.View(id).AsDecl()
	if doc := d.Doc(); doc.Valid() {
		for i := firstDocTok(p.t, doc.Index()); i <= doc.Index(); i++ {
			if p.t.Toks[i].Kind == token.DocComment {
				p.write(p.tok(i))
				p.newline()
			}
		}
		p.skipTo(doc.Index() + 1)
	}
	if attrs := d.Attrs(); attrs.Valid() {
		p.attrs(attrs)
	}
	if vis := d.Vis(); vis.Valid() {
		p.visibility(vis)
	}
	if d.Kind() == syntax.FnDecl {
		if mods := d.AsFnDecl().Mods(); mods != 0 {
			p.write(modsText(mods) + " ")
		}
	}
}

func (p *printer) attrs(list syntax.ListView) {
	var impls []syntax.NodeID
	for a := range list.Views() {
		if a.Kind() == syntax.UnsafeGuarantee {
			p.unsafeGuarantee(a.ID())
			p.newline()
			continue
		}
		attr := a.AsAttr()
		if attr.Path().Join(".") == "impl" {
			impls = attr.Args().AppendIDs(impls)
			continue
		}
		p.attr(attr)
		p.newline()
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
	if ret := f.Ret(); ret.Valid() {
		p.write(" -> ")
		p.typ(ret.ID())
	}
	if body := f.Body(); body.Valid() {
		p.write(" ")
		p.block(body.ID())
	}
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

func (p *printer) params(list syntax.NodeID) {
	p.write("(")
	for i, prm := range p.t.List(list).Enumerate() {
		if i > 0 {
			p.write(", ")
		}
		p.param(prm.AsParam())
	}
	p.write(")")
}

func (p *printer) param(prm syntax.ParamView) {
	flags := prm.Flags()
	if flags&syntax.FlagVariadic != 0 && !prm.Type().Valid() {
		p.write("...")
		return
	}
	for a := range prm.Attrs().Views() {
		if a.Kind() == syntax.UnsafeGuarantee {
			p.unsafeGuarantee(a.ID())
			p.write(" ")
			continue
		}
		p.attr(a.AsAttr())
		p.write(" ")
	}
	if flags&syntax.FlagMut != 0 {
		p.write("mut ")
	}
	if flags&syntax.FlagMove != 0 {
		p.write("move ")
	}
	p.write(prm.Name().Text())
	if typ := prm.Type(); typ.Valid() {
		p.write(": ")
		if flags&syntax.FlagVariadic != 0 {
			p.write("...")
		}
		p.typ(typ.ID())
	}
}
