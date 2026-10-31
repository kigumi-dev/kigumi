package mir

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

func (b *builder) blockExpr(blk *hir.Block) midir.LocalID {
	b.pushScope()
	dead := false
	for i, s := range blk.Stmts {
		b.stmt(s)
		if s.Kind == hir.ExprStmt && s.Expr.Type == sem.TyNever && (i < len(blk.Stmts)-1 || blk.Tail != nil) {
			dead = true
			break
		}
	}
	b.markExit(len(b.scopes) - 1)
	var result midir.LocalID
	if dead && blk.Tail != nil {
		result = b.unreached(blk.Node)
	} else if blk.Tail != nil {
		result = b.fullValue(func() midir.LocalID { return b.detach(b.exprMoved(blk.Tail)) })
	}
	if blk == b.body {
		b.closeTaskScope(blk.Node)
	}
	if blk == b.body && blk.Tail != nil {
		b.exitBodyScope(result, blk.Tail, blk.Node)
	} else {
		b.exitScope()
	}
	if blk.Tail == nil {
		if dead {
			result = b.unreached(blk.Node)
		} else {
			result = b.unit(blk.Node)
		}
	}
	return b.coerce(blk.Coercion, blk.Node, result)
}

func (b *builder) ifExpr(e *hir.Expr) midir.LocalID {
	var cond midir.LocalID
	var moves []guardMove
	// The bindings of the condition's `is` tests end with the then branch;
	// one a later `&&` test turned down stays with the enclosing scope.
	b.pushScope()
	bindings := b.scopes[len(b.scopes)-1]
	b.fullExpr(func() { cond, moves = b.cond(e.Cond) })
	result := b.temp(e.Type)
	then, els, join := b.newBlockAt(), b.newBlockAt(), b.newBlockAt()
	b.term(midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{cond}, Targets: []midir.BlockID{then, els}})
	b.cur = then
	b.markGuardMoves(moves)
	b.assignValue(result, b.blockExpr(e.Then), e.Node)
	b.unwindScope(bindings, false)
	b.jump(join)
	b.cur = els
	b.releaseGuardMoves(moves)
	b.scopes = b.scopes[:len(b.scopes)-1]
	outer := b.scopes[len(b.scopes)-1]
	b.handOverBindings(outer, bindings)
	if e.Else != nil {
		b.assignValue(result, b.expr(e.Else), e.Node)
	} else {
		b.assign(result, b.unit(e.Node))
	}
	b.jump(join)
	b.cur = join
	return result
}

// assignValue stores a branch's value into the result of an `if`. A branch
// of an `if` used as a statement may still produce a value (a foreign call
// whose pointer nobody reads); it is dropped, and the result stays Void.
func (b *builder) assignValue(dst, src midir.LocalID, n syntax.NodeID) {
	if b.fn.Locals[dst].Type == sem.TyUnit && b.fn.Locals[src].Type != sem.TyUnit && b.fn.Locals[src].Type != sem.TyNever {
		if b.r.Types.Kind(b.fn.Locals[src].Type) != sem.KRef {
			b.emit(midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{src}, Node: n})
		}
		b.assign(dst, b.unit(n))
		return
	}
	b.assign(dst, src)
}

func (b *builder) forExpr(e *hir.Expr) midir.LocalID {
	n := e.Node
	pattern, head := e.Pat, e.Cond
	valued := e.Type != sem.TyUnit && e.Type != sem.TyNever
	var result midir.LocalID
	if valued {
		result = b.temp(e.Type)
	}
	cond, loop, next, exit := b.newBlockAt(), b.newBlockAt(), b.newBlockAt(), b.newBlockAt()
	// The loop's bookkeeping temporaries (and an iterable produced for the
	// loop) live in a scope of their own, so break, return and the normal
	// exit all release them.
	b.pushScope()
	var cur forCursor
	var headBound []midir.LocalID
	var headMoves []guardMove
	if pattern != nil {
		cur = b.forCursorFor(e)
	}
	b.jump(cond)
	b.cur = cond
	switch {
	case pattern != nil:
		b.term(midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{cur.test(n)}, Targets: []midir.BlockID{loop, exit}})
	case head != nil:
		var c midir.LocalID
		b.fullExpr(func() { c, headBound, headMoves = b.loopHead(head) })
		b.term(midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{c}, Targets: []midir.BlockID{loop, exit}})
	default:
		b.jump(loop)
	}
	b.cur = loop
	b.loops = append(b.loops, &loopCtx{cont: next, exit: exit, depth: len(b.scopes), valued: valued, result: result})
	b.pushScope()
	b.markGuardMoves(headMoves)
	if pattern != nil && (pattern.Kind != hir.PatWild || !b.r.IsCopy(pattern.Type)) {
		b.bindPattern(pattern, cur.elem(n))
	}
	b.blockExpr(e.Block)
	b.exitScope()
	b.loops = b.loops[:len(b.loops)-1]
	b.jump(next)
	b.cur = next
	b.dropLoopBound(headBound)
	if pattern != nil {
		cur.advance(n, cond, exit)
	}
	b.jump(cond)
	b.cur = exit
	b.releaseGuardMoves(headMoves)
	b.exitScope()
	if e.Type == sem.TyNever {
		// Nothing breaks out of this loop; what follows never runs.
		b.term(midir.Term{Op: midir.TermUnreachable})
		return b.never(n)
	}
	if valued {
		return result
	}
	return b.unit(n)
}

func (b *builder) markRetain(v midir.LocalID) {
	insts := b.block().Insts
	if n := len(insts); n > 0 && insts[n-1].Op == midir.OpCopy && insts[n-1].Dst == v {
		insts[n-1].Retain = true
	}
}

// unreached stands for the value of a block whose later code is dead, in the orphan block the diverging statement left.
func (b *builder) unreached(n syntax.NodeID) midir.LocalID {
	return b.emit(midir.Inst{Op: midir.OpUnit, Dst: b.temp(sem.TyNever), Node: n})
}
