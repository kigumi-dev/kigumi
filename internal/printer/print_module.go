package printer

import "kigumi/internal/syntax"

func (p *printer) constDecl(id syntax.NodeID) {
	p.prefix(id)
	d := p.t.View(id).AsConstDecl()
	p.write("const " + d.Name().Text())
	if typ := d.Type(); typ.Valid() {
		p.write(": ")
		p.typ(typ.ID())
	}
	p.write(" = ")
	p.expr(d.Value().ID())
}

func (p *printer) staticDecl(id syntax.NodeID) {
	p.prefix(id)
	d := p.t.View(id).AsStaticDecl()
	p.write("static ")
	if d.Flags()&syntax.FlagMut != 0 {
		p.write("mut ")
	}
	p.write(d.Name().Text() + ": ")
	p.typ(d.Type().ID())
	if value := d.Value(); value.Valid() {
		p.write(" = ")
		p.expr(value.ID())
	}
}

func (p *printer) contextKeyDecl(id syntax.NodeID) {
	p.prefix(id)
	d := p.t.View(id).AsContextKeyDecl()
	p.write("context " + d.Name().Text() + ": ")
	p.typ(d.Type().ID())
}

func (p *printer) importDecl(id syntax.NodeID) {
	d := p.t.View(id).AsImportDecl()
	if vis := d.Vis(); vis.Valid() {
		p.visibility(vis)
	}
	p.write("import ")
	binding := d.Binding()
	if binding.Kind() == syntax.List {
		names := p.sortedNames(binding.AsList().AppendIDs(nil))
		p.write("{")
		for i, nm := range names {
			if i > 0 {
				p.write(", ")
			}
			v := p.t.View(nm)
			if v.Kind() == syntax.ImportAlias {
				a := v.AsImportAlias()
				p.write(a.Name().Name().Text() + " as " + a.Alias().Name().Text())
				continue
			}
			p.write(v.Tok().Text())
		}
		p.write("}")
	} else {
		p.write(binding.Tok().Text())
	}
	p.write(" from " + d.Path().Join("/"))
}

func (p *printer) interfaceDecl(id syntax.NodeID) {
	p.prefix(id)
	d := p.t.View(id).AsInterfaceDecl()
	p.write("interface " + d.Name().Text())
	if g := d.Generics(); g.Valid() {
		p.genericParams(g.ID())
	}
	p.write(" ")
	p.declList(d.Members().ID())
}

func (p *printer) abiBlock(id syntax.NodeID) {
	d := p.t.View(id).AsAbiBlock()
	p.write(d.Tok().Text() + "(")
	for i, a := range d.Abis().Enumerate() {
		if i > 0 {
			p.write(", ")
		}
		p.write(a.Tok().Text())
	}
	p.write(") ")
	p.declList(d.Decls().ID())
}
