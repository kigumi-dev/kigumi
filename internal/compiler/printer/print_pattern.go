package printer

import "kigumi/internal/compiler/syntax"

func (p *printer) pattern(id syntax.NodeID) {
	v := p.t.View(id)
	switch v.Kind() {
	case syntax.PatWildcard, syntax.PatBind:
		p.write(v.Tok().Text())
	case syntax.PatLit:
		p.expr(v.AsPatLit().Lit().ID())
	case syntax.PatCtor:
		c := v.AsPatCtor()
		p.write(c.Path().Join("."))
		if args := c.Args(); args.Valid() {
			p.write("(")
			for i, sub := range args.Enumerate() {
				if i > 0 {
					p.write(", ")
				}
				p.pattern(sub.ID())
			}
			p.write(")")
		}
	case syntax.PatTuple:
		items := v.AsPatTuple()
		p.write("(")
		for i, sub := range items.Enumerate() {
			if i > 0 {
				p.write(", ")
			}
			p.pattern(sub.ID())
		}
		if items.Len() == 1 {
			p.write(",")
		}
		p.write(")")
	case syntax.PatRange:
		r := v.AsPatRange()
		p.expr(r.Lo().ID())
		p.write(r.Op().Text())
		p.expr(r.Hi().ID())
	case syntax.PatOr:
		for i, alt := range v.AsPatOr().Enumerate() {
			if i > 0 {
				p.write(" | ")
			}
			p.pattern(alt.ID())
		}
	case syntax.PatRecord:
		r := v.AsPatRecord()
		p.write(r.Path().Join(".") + " {")
		p.newline()
		p.indent++
		for fv := range r.Fields().Views() {
			f := fv.AsPatField()
			p.write(f.Name().Text())
			if sub := f.Sub(); sub.Valid() {
				p.write(": ")
				p.pattern(sub.ID())
			}
			p.newline()
		}
		p.indent--
		p.write("}")
	}
}
