package printer

import "kigumi/internal/compiler/syntax"

func (p *printer) constDecl(id syntax.NodeID) {
	p.prefix(id)
	d := p.t.View(id).AsConstDecl()
	p.write("const ")
	p.inlineBlocks(uint32(p.cm.next), d.Name().Index(), false, true)
	p.write(d.Name().Text())
	if typ := d.Type(); typ.Valid() {
		p.write(": ")
		p.typ(typ.ID())
	}
	p.initExpr("=", d.Value().ID())
}

func (p *printer) staticDecl(id syntax.NodeID) {
	p.prefix(id)
	d := p.t.View(id).AsStaticDecl()
	p.write("static ")
	if d.Flags()&syntax.FlagMut != 0 {
		p.write("mut ")
	}
	p.inlineBlocks(uint32(p.cm.next), d.Name().Index(), false, true)
	p.write(d.Name().Text() + ": ")
	p.typ(d.Type().ID())
	if value := d.Value(); value.Valid() {
		p.initExpr("=", value.ID())
	}
}

func (p *printer) contextKeyDecl(id syntax.NodeID) {
	p.prefix(id)
	d := p.t.View(id).AsContextKeyDecl()
	p.write("context ")
	p.inlineBlocks(uint32(p.cm.next), d.Name().Index(), false, true)
	p.write(d.Name().Text() + ": ")
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
		p.importNames(id, binding.AsList().AppendIDs(nil))
	} else {
		p.inlineBlocks(uint32(p.cm.next), binding.Tok().Index(), false, true)
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
