package printer

import (
	"kigumi/internal/syntax"
)

func (p *printer) block(id syntax.NodeID) {
	stmts := p.t.List(id)
	if stmts.Len() == 0 {
		p.emptyBlock(id)
		return
	}
	p.write("{")
	p.newline()
	p.indent++
	for c := stmts.Cursor(); c.Next(); {
		s := c.ID()
		if c.Index() > 0 && p.blankBefore(int(p.firstTok(s))) {
			p.blank()
		}
		p.flushComments(p.firstTok(s))
		p.withIndent(func() { p.stmt(s) })
		p.endLine(s)
	}
	p.flushComments(p.closingBrace(id))
	p.indent--
	p.write("}")
}

// endLine finishes a statement line, keeping a trailing comment attached.
func (p *printer) endLine(s syntax.NodeID) {
	last := p.lastTok(s)
	c := p.peekTrailing(last)
	if c == "" {
		p.newline()
		return
	}
	p.write(" " + c)
	p.newline()
	p.flushComments(last + 1)
	p.cm.next = int(last) + 2
}

func (p *printer) stmt(id syntax.NodeID) {
	v := p.t.View(id)
	switch v.Kind() {
	case syntax.LetStmt:
		s := v.AsLet()
		p.write("let ")
		if s.Flags()&syntax.FlagMut != 0 {
			p.write("mut ")
		}
		p.pattern(s.Pattern().ID())
		if typ := s.Type(); typ.Valid() {
			p.write(": ")
			p.typ(typ.ID())
		}
		p.write(" = ")
		p.expr(s.Init().ID())
		if els := s.Else(); els.Valid() {
			p.write(" else ")
			p.block(els.ID())
		}
	case syntax.ExprStmt:
		p.expr(v.AsExprStmt().Expr().ID())
	case syntax.AssignStmt:
		s := v.AsAssign()
		p.expr(s.Target().ID())
		p.write(" " + s.Op().Text() + " ")
		p.expr(s.Value().ID())
	case syntax.DeferStmt:
		p.write("defer ")
		p.cleanup(v.AsDefer().Body().ID())
	case syntax.DeferFailStmt:
		p.write("defer(fail) ")
		p.cleanup(v.AsDeferFail().Body().ID())
	default:
		p.expr(id)
	}
}

func (p *printer) cleanup(body syntax.NodeID) {
	if p.t.Kind(body) == syntax.Block {
		p.block(body)
		return
	}
	p.stmt(body)
}

func (p *printer) ifExpr(id syntax.NodeID) {
	s := p.t.View(id).AsIf()
	p.write("if ")
	p.expr(s.Cond().ID())
	p.write(" ")
	p.block(s.Then().ID())
	p.elseClause(s.Else().ID())
}

func (p *printer) ifLet(id syntax.NodeID) {
	s := p.t.View(id).AsIfLet()
	p.write("if let ")
	if s.Flags()&syntax.FlagMut != 0 {
		p.write("mut ")
	}
	p.pattern(s.Pattern().ID())
	p.write(" = ")
	p.expr(s.Init().ID())
	if guard := s.Guard(); guard.Valid() {
		p.write("; ")
		p.expr(guard.ID())
	}
	p.write(" ")
	p.block(s.Then().ID())
	p.elseClause(s.Else().ID())
}

func (p *printer) elseClause(e syntax.NodeID) {
	if e == 0 {
		return
	}
	p.write(" else ")
	if p.t.Kind(e) == syntax.Block {
		p.block(e)
		return
	}
	p.expr(e)
}

func (p *printer) matchExpr(id syntax.NodeID) {
	m := p.t.View(id).AsMatch()
	p.write("match ")
	p.expr(m.Scrutinee().ID())
	p.write(" {")
	p.newline()
	p.indent++
	for v := range m.Arms().Views() {
		arm := v.AsArm()
		p.flushComments(p.firstTok(arm.ID()))
		p.pattern(arm.Pattern().ID())
		if guard := arm.Guard(); guard.Valid() {
			p.write(" if ")
			p.expr(guard.ID())
		}
		p.write(" -> ")
		p.expr(arm.Body().ID())
		p.newline()
	}
	p.indent--
	p.write("}")
}

func (p *printer) forExpr(id syntax.NodeID) {
	f := p.t.View(id).AsFor()
	p.write("for ")
	if pat := f.Pattern(); pat.Valid() {
		p.pattern(pat.ID())
		p.write(" in ")
	}
	if head := f.Head(); head.Valid() {
		p.expr(head.ID())
		p.write(" ")
	}
	p.block(f.Body().ID())
}
