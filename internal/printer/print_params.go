package printer

import (
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

// paramsStack reports a parameter list that cannot stay on one line: a default laid out over several lines, or a comment between parameters.
func (p *printer) paramsStack(list syntax.NodeID) bool {
	items := p.t.List(list)
	if items.Len() == 0 {
		return false
	}
	for prm := range items.Views() {
		if def := prm.AsParam().Default(); def.Valid() && (p.multiline(def.ID()) || p.breaksLines(def.View)) {
			return true
		}
	}
	first, last := p.t.View(items.ItemID(0)).Tok().Index(), p.t.View(items.ItemID(items.Len()-1)).Tok().Index()
	for first > 0 && p.t.Toks[first-1].Kind != token.LParen {
		first--
	}
	for i := first; i < last; i++ {
		if k := p.t.Toks[i].Kind; k == token.Comment || k == token.BlockComment {
			return true
		}
	}
	return false
}

func (p *printer) paramsVertical(list syntax.NodeID) {
	p.write("(")
	p.newline()
	p.indent++
	for prm := range p.t.List(list).Views() {
		p.flushComments(prm.Tok().Index())
		p.withIndent(func() { p.param(prm.AsParam()) })
		p.write(",")
		p.newline()
	}
	p.indent--
	p.write(")")
}

func (p *printer) params(list syntax.NodeID) {
	if p.paramsStack(list) {
		p.paramsVertical(list)
		return
	}
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
		if flags&syntax.FlagEscaping != 0 {
			p.write("escaping ")
		}
		p.typ(typ.ID())
	}
	if def := prm.Default(); def.Valid() {
		p.write(" = ")
		p.withIndent(func() { p.expr(def.ID()) })
	}
}
