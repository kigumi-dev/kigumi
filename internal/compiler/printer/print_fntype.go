package printer

import "kigumi/internal/compiler/syntax"

func (p *printer) fnType(id syntax.NodeID) {
	f := p.t.View(id).AsTypeFn()
	flags := f.Flags()
	if flags&syntax.FlagEscaping != 0 {
		p.write("escaping ")
	}
	if abi := f.Abi(); abi.Valid() {
		p.write("extern(" + abi.Text() + ") ")
	}
	if mods := f.Mods(); mods != 0 {
		p.write(modsText(mods) + " ")
	}
	p.write("fn(")
	params := f.Params()
	cVariadic := flags&syntax.FlagVariadic != 0 && f.Abi().Valid()
	for i, a := range params.Enumerate() {
		if i > 0 {
			p.write(", ")
		}
		if flags&syntax.FlagVariadic != 0 && !cVariadic && i == params.Len()-1 {
			p.write("...")
		}
		if flags&(syntax.FlagEscapingParam0<<i) != 0 {
			p.write("escaping ")
		}
		p.typ(a.ID())
	}
	if cVariadic {
		if params.Len() > 0 {
			p.write(", ")
		}
		p.write("...")
	}
	p.write(") -> ")
	p.typ(f.Ret().ID())
}
