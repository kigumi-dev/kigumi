package mir

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
)

const guardFailed = "guard-failed"

// guard lowers g as an `if` condition, so a temporary its `is` bindings borrow lives until the arm's scope ends.
func (b *builder) guard(g *hir.Expr, run, orElse midir.BlockID) []guardMove {
	var c midir.LocalID
	var moves []guardMove
	b.fullExpr(func() { c, moves = b.cond(g) })
	fail := b.newBlockAt()
	b.term(midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{c}, Targets: []midir.BlockID{run, fail}})
	b.cur = fail
	b.releaseGuardMoves(moves)
	b.unwindScope(b.scopes[len(b.scopes)-1], false)
	b.jump(orElse)
	return moves
}

func (b *builder) releaseGuardMoves(moves []guardMove) {
	for _, m := range moves {
		if m.take == nil {
			b.emit(midir.Inst{Op: midir.OpNop, Args: []midir.LocalID{m.place}, Node: m.node, Guarded: true, Str: guardFailed})
		}
	}
}

func (b *builder) markGuardMoves(moves []guardMove) {
	for _, m := range moves {
		if m.take != nil {
			b.takeScrutinee(m.place, m.take, true)
			continue
		}
		if b.wholeLocal(m.place) {
			// The binding holds its own reference; the passed guard empties the local it was copied from.
			t := b.emit(midir.Inst{Op: midir.OpMove, Dst: b.temp(b.fn.Locals[m.place].Type), Args: []midir.LocalID{m.place}, Node: m.node, Guarded: true})
			b.emit(midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{t}, Node: m.node})
			continue
		}
		b.emit(midir.Inst{Op: midir.OpNop, Args: []midir.LocalID{m.place, m.bind}, Node: m.node, Guarded: true})
	}
}

func (b *builder) wholeLocal(v midir.LocalID) bool {
	l := b.fn.Locals[v]
	return l.Ent != 0 && !l.Cell && !l.Borrowed && !b.shared[v] && !b.r.IsCopy(l.Type)
}

// readsCell is a scrutinee temporary read out of a move-only cell local; its guarded binding takes the cell's content once the guard passes.
func (b *builder) readsCell(v midir.LocalID) bool {
	for _, blk := range b.fn.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpCellGet && in.Dst == v {
				return trackedCell(b.p.Program, b.fn.Locals[in.Args[0]])
			}
		}
	}
	return false
}
