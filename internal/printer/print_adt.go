package printer

import "kigumi/internal/syntax"

func (p *printer) variants(body syntax.NodeID) {
	vs := p.t.List(body)
	multi := vs.Len() > 3 || p.multiline(body)
	for i, vv := range vs.Enumerate() {
		switch {
		case multi:
			p.newline()
			p.indent++
			p.write("| ")
			p.indent--
		case i > 0:
			p.write(" | ")
		case vs.Len() == 1:
			// A lone variant keeps its bar: without it the declaration
			// reads back as an alias of that name.
			p.write(" | ")
		default:
			p.write(" ")
		}
		v := vv.AsVariant()
		p.write(v.Name().Text())
		if fields := v.Fields(); fields.Valid() {
			p.write("(")
			for j, fv := range fields.Enumerate() {
				if j > 0 {
					p.write(", ")
				}
				f := fv.AsVariantField()
				if f.Flags()&syntax.FlagNamed != 0 {
					p.write(f.Name().Text() + ": ")
				}
				p.typ(f.Type().ID())
			}
			p.write(")")
		}
	}
}
