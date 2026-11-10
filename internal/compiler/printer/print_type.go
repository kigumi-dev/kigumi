package printer

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
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
		p.write(" resource")
		p.inlineBlockComments(body.Tok().Index(), token.LBrace)
		p.write(" ")
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

func (p *printer) typ(id syntax.NodeID) {
	if id == 0 {
		return
	}
	p.leadingInline(id)
	p.typNode(id)
	p.trailingInline(id)
}

func (p *printer) typNode(id syntax.NodeID) {
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
		lifetime := ref.Op().Kind() == token.Lifetime
		if lifetime {
			p.write(ref.Op().Text() + " ")
		}
		if ref.Flags()&syntax.FlagMut != 0 {
			p.write("mut ")
		} else if !lifetime {
			p.prefixSep(ref.Pointee().ID())
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
