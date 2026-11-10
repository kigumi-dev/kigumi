package printer

import "kigumi/internal/compiler/syntax"

// wsCall keeps `f x y` only for a bare identifier callee whose arguments stay on one line; every other shape becomes an ordinary call.
func (p *printer) wsCall(id syntax.NodeID) {
	c := p.t.View(id).AsWsCall()
	callee := c.Callee()
	args := c.Args()
	ids := args.AppendIDs(nil)
	if callee.Kind() == syntax.Ident && (args.Len() == 1 || !p.anyMultiline(args)) {
		if p.wsArgsBreak(id, callee.ID(), ids) {
			p.expr(callee.ID())
			p.verticalExprList(ids, p.lastTok(id)+1)
			return
		}
		p.expr(callee.ID())
		prev := p.lastTok(callee.ID())
		for _, a := range ids {
			p.write(" ")
			p.inlineBlocks(prev+1, p.firstTok(a), false, true)
			p.withIndent(func() { p.expr(a) })
			prev = p.lastTok(a)
		}
		return
	}
	p.expr(callee.ID())
	p.write("(")
	p.exprList(args.ID())
	p.write(")")
}

// wsArgsBreak reports whether a comment inside a space-separated call forces
// the ordinary `f(a, b)` form: only line comments do, because they would end
// the call's line.
func (p *printer) wsArgsBreak(call, callee syntax.NodeID, ids []syntax.NodeID) bool {
	prev := p.lastTok(callee)
	for _, a := range ids {
		if p.noteLineBreak(prev+1, p.firstTok(a)) {
			return true
		}
		prev = p.lastTok(a)
	}
	return p.noteLineBreak(prev+1, p.lastTok(call)+1)
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
