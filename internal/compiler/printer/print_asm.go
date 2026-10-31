package printer

import "kigumi/internal/compiler/syntax"

// asmBlock prints `asm { ... }` one entry per line in source order.
func (p *printer) asmBlock(id syntax.NodeID) {
	lines := p.t.View(id).AsAsm().Lines()
	if lines.Len() == 0 {
		p.write("asm {}")
		return
	}
	p.write("asm {")
	p.newline()
	p.indent++
	for c := lines.Cursor(); c.Next(); {
		l := c.ID()
		if c.Index() > 0 && p.blankBefore(int(p.firstTok(l))) {
			p.blank()
		}
		p.flushComments(p.firstTok(l))
		p.withIndent(func() { p.asmLine(l) })
		p.endLine(l)
	}
	p.indent--
	p.write("}")
}

func (p *printer) asmLine(l syntax.NodeID) {
	v := p.t.View(l)
	switch v.Kind() {
	case syntax.AsmTemplate:
		p.expr(v.AsAsmTemplate().Text().ID())
	case syntax.AsmOperand:
		o := v.AsAsmOperand()
		if label := o.Label(); label.Valid() {
			p.write(label.Text() + ": ")
		}
		p.write(o.Dir().Text())
		if reg := o.Reg(); reg.Valid() {
			p.write("(" + reg.Text() + ")")
		}
		if expr := o.Expr(); expr.Valid() {
			p.write(" ")
			p.expr(expr.ID())
		}
	case syntax.AsmClobber, syntax.AsmOptions:
		names := p.asmNames(v)
		p.write(v.Tok().Text())
		for i, it := range names.Enumerate() {
			if i == 0 {
				p.write(" ")
			} else {
				p.write(", ")
			}
			if it.Kind() == syntax.AsmAbi {
				p.write("abi(" + it.Tok().Text() + ")")
			} else {
				p.write(it.Tok().Text())
			}
		}
	}
}

func (p *printer) asmNames(v syntax.View) syntax.ListView {
	if v.Kind() == syntax.AsmClobber {
		return v.AsAsmClobber().Names()
	}
	return v.AsAsmOptions().Names()
}
