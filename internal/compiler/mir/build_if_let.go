package mir

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
)

func (b *builder) ifLet(e *hir.Expr) midir.LocalID {
	init := b.expr(e.Cond)
	b.pushScope()
	b.ownTemp(init)
	b.ownBorrowedTemp(e.Cond, init)
	result := b.temp(e.Type)
	then, els, join := b.newBlockAt(), b.newBlockAt(), b.newBlockAt()
	matched := b.newBlockAt()
	cond := b.testPattern(e.Pat, init)
	b.term(midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{cond}, Targets: []midir.BlockID{matched, els}})
	b.cur = matched
	b.pushScope()
	moves := b.bindGuarded(e.Pat, init, e.Guard != nil)
	if e.Guard != nil {
		moves = append(moves, b.guard(e.Guard, then, els)...)
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
