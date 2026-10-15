package printer

import "kigumi/internal/syntax"

func (p *printer) recordLit(id syntax.NodeID) {
	r := p.t.View(id).AsRecordLit()
	p.expr(r.Head().ID())
	entries := r.Fields()
	if entries.Len() == 1 {
		p.write(" { ")
		p.recordEntry(entries.ItemID(0))
		p.write(" }")
		return
	}
	p.write(" {")
	p.newline()
	p.indent++
	for e := range entries.IDs() {
		p.withIndent(func() { p.recordEntry(e) })
		p.newline()
	}
	p.indent--
	p.write("}")
}

func (p *printer) recordEntry(e syntax.NodeID) {
	v := p.t.View(e)
	if v.Kind() == syntax.Spread {
		p.write("..")
		p.expr(v.AsSpread().Value().ID())
		return
	}
	f := v.AsFieldInit()
	p.write(f.Name().Text() + ": ")
	p.expr(f.Value().ID())
}

// tupleLit prints `(a, b, ...)`; a single element keeps its trailing comma
// so it reparses as a tuple and not a grouped expression.
func (p *printer) tupleLit(id syntax.NodeID) {
	p.write("(")
	p.exprList(id)
	if p.t.List(id).Len() == 1 {
		p.write(",")
	}
	p.write(")")
}

func (p *printer) lambda(id syntax.NodeID) {
	l := p.t.View(id).AsLambda()
	p.params(l.Params().ID())
	p.write(" => ")
	p.expr(l.Body().ID())
}
