package mir

import "kigumi/internal/hir"

func (b *builder) ifLet(e *hir.Expr) LocalID {
	init := b.expr(e.Cond)
	b.pushScope()
	b.ownTemp(init)
	b.ownBorrowedTemp(e.Cond, init)
	result := b.temp(e.Type)
	then, els, join := b.newBlockAt(), b.newBlockAt(), b.newBlockAt()
	matched := b.newBlockAt()
	cond := b.testPattern(e.Pat, init)
	b.term(Term{Op: TermBranch, Args: []LocalID{cond}, Targets: []BlockID{matched, els}})
	b.cur = matched
	b.pushScope()
	moves := b.bindGuarded(e.Pat, init, e.Guard != nil)
	if e.Guard != nil {
		gv := b.expr(e.Guard)
		fail := b.newBlockAt()
		b.term(Term{Op: TermBranch, Args: []LocalID{gv}, Targets: []BlockID{then, fail}})
		b.cur = fail
		b.unwindScope(b.scopes[len(b.scopes)-1], false)
		b.jump(els)
	} else {
		b.jump(then)
	}
	b.cur = then
	b.markGuardMoves(moves)
	b.assignValue(result, b.blockExpr(e.Then), e.Node)
	b.exitScope()
	b.jump(join)
	b.cur = els
	if e.Else != nil {
		b.assignValue(result, b.expr(e.Else), e.Node)
	} else {
		b.assign(result, b.unit(e.Node))
	}
	b.jump(join)
	b.cur = join
	b.exitScope()
	return result
}
