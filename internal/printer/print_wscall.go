package printer

import "kigumi/internal/syntax"

// wsCall keeps `f x y` only for a bare identifier callee whose arguments stay on one line; every other shape becomes an ordinary call.
func (p *printer) wsCall(id syntax.NodeID) {
	c := p.t.View(id).AsWsCall()
	callee := c.Callee()
	args := c.Args()
	if callee.Kind() == syntax.Ident && (args.Len() == 1 || !p.anyMultiline(args)) {
		p.expr(callee.ID())
		for a := range args.Views() {
			p.write(" ")
			p.withIndent(func() { p.expr(a.ID()) })
		}
		return
	}
	p.expr(callee.ID())
	p.write("(")
	p.exprList(args.ID())
	p.write(")")
}

func (p *printer) anyMultiline(list syntax.ListView) bool {
	for a := range list.Views() {
		if p.multiline(a.ID()) || p.breaksLines(a) {
			return true
		}
	}
	return false
}

// breaksLines reports an expression the printer always lays out over several lines, however the source wrote it.
func (p *printer) breaksLines(v syntax.View) bool {
	switch v.Kind() {
	case syntax.RecordLit:
		if v.AsRecordLit().Fields().Len() != 1 {
			return true
		}
	case syntax.Block, syntax.IfExpr, syntax.IfLetExpr, syntax.MatchExpr, syntax.ForExpr, syntax.ComptimeExpr, syntax.UnsafeExpr, syntax.AsmExpr:
		return true
	}
	for c := range v.Children() {
		if p.breaksLines(c) {
			return true
		}
	}
	return false
}
