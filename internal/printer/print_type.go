package printer

import (
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

func (p *printer) typeDecl(id syntax.NodeID) {
	p.prefix(id)
	d := p.t.View(id).AsTypeDecl()
	p.write("type " + d.Name().Text())
	if g := d.Generics(); g.Valid() {
		p.genericParams(g.ID())
	}
	if layout := d.Layout(); layout.Valid() {
		p.write(" layout(")
		p.layoutArgs(layout.Args())
		p.write(")")
	}
	body := d.Body()
	if !body.Valid() {
		return
	}
	p.write(" =")
	switch body.Kind() {
	case syntax.ResourceBody:
		p.write(" resource ")
		p.fields(body.AsResourceBody().ListView)
	case syntax.RecordBody:
		p.write(" ")
		p.fields(body.AsRecordBody().ListView)
	case syntax.AdtBody:
		p.variants(body.ID())
	case syntax.AliasBody:
		p.write(" alias ")
		p.typ(body.AsAliasBody().Type().ID())
	case syntax.DefinedBody:
		p.write(" ")
		p.typ(body.AsDefinedBody().Type().ID())
	}
}

// layoutArgs prints a `layout(...)` clause's arguments: bare names (`C`,
// `packed`) and `name: value` args (`align: N`) alike.
func (p *printer) layoutArgs(list syntax.ListView) {
	for i, a := range list.Enumerate() {
		if i > 0 {
			p.write(", ")
		}
		if a.Kind() == syntax.FieldInit {
			p.recordEntry(a.ID())
			continue
		}
		p.expr(a.ID())
	}
}

func (p *printer) fields(fields syntax.ListView) {
	if fields.Len() == 0 {
		p.write("{}")
		return
	}
	p.write("{")
	p.newline()
	p.indent++
	for i, fv := range fields.Enumerate() {
		f := fv.AsField()
		if i > 0 && (f.Metadata().Valid() || fields.At(i-1).AsField().Metadata().Valid()) {
			p.blank()
		}
		p.flushComments(p.firstTok(f.ID()))
		if vis := f.Vis(); vis.Valid() {
			p.visibility(vis)
		}
		if f.Flags()&syntax.FlagMut != 0 {
			p.write("mut ")
		}
		p.write(f.Name().Text() + " ")
		p.typ(f.Type().ID())
		if meta := f.Metadata(); meta.Valid() {
			p.write(" ")
			p.metadata(meta)
		}
		p.newline()
	}
	p.indent--
	p.write("}")
}

func (p *printer) metadata(meta syntax.MetadataView) {
	p.write("{")
	p.newline()
	p.indent++
	for iv := range meta.Views() {
		item := iv.AsMetaItem()
		p.write(item.Path().Join("."))
		if args := item.Args(); args.Valid() {
			p.write("(")
			p.exprList(args.ID())
			p.write(")")
		}
		p.newline()
	}
	p.indent--
	p.write("}")
}

func (p *printer) typ(id syntax.NodeID) {
	v := p.t.View(id)
	switch v.Kind() {
	case syntax.TypePath:
		tp := v.AsTypePath()
		p.write(tp.Path().Join("."))
		if args := tp.Args(); args.Valid() {
			p.write("[")
			for i, a := range args.Enumerate() {
				if i > 0 {
					p.write(", ")
				}
				switch a.Kind() {
				case syntax.GenericParam:
					p.genericParam(a.AsGenericParam())
				case syntax.IntLit, syntax.BoolLit:
					p.expr(a.ID())
				case syntax.TypeLifetime:
					p.write(a.Tok().Text())
				default:
					p.typ(a.ID())
				}
			}
			p.write("]")
		}
	case syntax.TypeOptional:
		p.typ(v.AsTypeOptional().Inner().ID())
		p.write("?")
	case syntax.TypeResult:
		p.typ(v.AsTypeResult().Inner().ID())
		p.write("!")
	case syntax.TypePtr:
		ptr := v.AsTypePtr()
		if ptr.Flags()&syntax.FlagMut != 0 {
			p.write("*mut ")
		} else {
			p.write("*const ")
		}
		p.typ(ptr.Pointee().ID())
	case syntax.TypeRef:
		ref := v.AsTypeRef()
		p.write("&")
		if ref.Op().Kind() == token.Lifetime {
			p.write(ref.Op().Text() + " ")
		}
		if ref.Flags()&syntax.FlagMut != 0 {
			p.write("mut ")
		}
		p.typ(ref.Pointee().ID())
	case syntax.Paren:
		p.write("(")
		p.typ(v.AsParen().Inner().ID())
		p.write(")")
	case syntax.TypeTuple:
		items := v.AsTypeTuple()
		p.write("(")
		for i, a := range items.Enumerate() {
			if i > 0 {
				p.write(", ")
			}
			p.typ(a.ID())
		}
		if items.Len() == 1 {
			p.write(",")
		}
		p.write(")")
	case syntax.TypeFn:
		p.fnType(id)
	}
}
