package build

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

func (b *builder) matchExpr(e *hir.Expr) midir.LocalID {
	scrutinee := b.fullValue(func() midir.LocalID { return b.expr(e.Args[0]) })
	// The scrutinee's scope opens after it is assigned, so every exit of
	// the arms (including early ones) drops a live temporary.
	b.pushScope()
	b.ownTemp(scrutinee)
	b.ownBorrowedTemp(e.Args[0], scrutinee)
	result := b.temp(e.Type)
	join := b.newBlockAt()
	for _, arm := range e.Arms {
		body, next := b.newBlockAt(), b.newBlockAt()
		cond := b.testPattern(arm.Pat, scrutinee)
		b.term(midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{cond}, Targets: []midir.BlockID{body, next}})
		b.cur = body
		b.pushScope()
		moves := b.bindGuarded(arm.Pat, scrutinee, arm.Guard != nil)
		if arm.Guard != nil {
			run := b.newBlockAt()
			moves = append(moves, b.guard(arm.Guard, run, next)...)
			b.cur = run
			b.markGuardMoves(moves)
		}
		b.assignValue(result, b.armBody(arm.Body), e.Node)
		b.exitScope()
		b.jump(join)
		b.cur = next
	}
	b.emit(midir.Inst{Op: midir.OpPanic, Str: "no match arm matched", Node: e.Node, NoTrap: true})
	b.term(midir.Term{Op: midir.TermUnreachable})
	b.cur = join
	b.exitScope()
	return result
}

// bindPattern declares the bindings of a pattern that already matched.
func (b *builder) bindPattern(p *hir.Pat, v midir.LocalID) {
	switch p.Kind {
	case hir.PatBind:
		id := b.declare(p.Ent)
		if b.fn.Locals[id].Cell {
			if b.shared[v] {
				v = b.emit(midir.Inst{Op: midir.OpCopy, Dst: b.temp(b.fn.Locals[v].Type), Args: []midir.LocalID{v}, Node: p.Node, Share: true})
			}
			b.emit(midir.Inst{Op: midir.OpNewCell, Dst: id, Args: []midir.LocalID{v}, Node: p.Node})
			return
		}
		b.bindLocal(id, v, p.Node)
		b.ownIfCounted(id)
	case hir.PatCtor:
		if p.IsType {
			if len(p.Subs) == 1 {
				b.bindPattern(p.Subs[0], b.emit(midir.Inst{Op: midir.OpUnbox, Dst: b.temp(p.Subs[0].Type), Args: []midir.LocalID{v}, Node: p.Node}))
			}
			return
		}
		for i, s := range p.Subs {
			if s.Kind == hir.PatWild {
				continue
			}
			if s.Type == sem.TyNever {
				b.bindPattern(s, b.unreached(s.Node))
				continue
			}
			b.bindPattern(s, b.emit(midir.Inst{Op: midir.OpPayload, Dst: b.temp(s.Type), Args: []midir.LocalID{v}, Index: i, Node: s.Node}))
		}
	case hir.PatRecord:
		for _, f := range p.Fields {
			var val midir.LocalID
			if b.takesField(p, f, v) {
				val = b.emit(midir.Inst{Op: midir.OpFieldMove, Dst: b.temp(f.Type), Args: []midir.LocalID{v}, Index: f.Index, Node: f.Node})
			} else {
				val = b.fieldOf(f, v)
			}
			if f.Sub != nil {
				b.bindPattern(f.Sub, val)
				continue
			}
			id := b.declare(f.Bind)
			b.bindLocal(id, val, f.Node)
			b.ownIfCounted(id)
		}
	}
}

// bindLocal initializes a pattern binding: a `&mut ScalarT` binding of a
// plain ScalarT payload gets a cell, since storePlace's OpCellSet
// trusts this static shape and a bare OpBorrow alias would hand it a
// non-cell object; a `&U` binding of a U payload otherwise aliases it, and
// anything else copies.
func (b *builder) bindLocal(id, v midir.LocalID, n syntax.NodeID) {
	tt := b.r.Types
	idT := b.fn.Locals[id].Type
	vNotRef := tt.Kind(b.fn.Locals[v].Type) != sem.KRef
	if _, ok := b.r.ScalarRefElem(idT); ok && vNotRef {
		// v's own type is the unwrapped payload (binding mode strips
		// the pattern's own type to the scrutinee), so it needs the same
		// OpBorrow retype + OpShare retain as an explicit `&mut expr` of a
		// place (build_expr.go's hir.Borrow) before OpNewCell can take it:
		// OpNewCell requires its argument's static type to already match id's.
		ref := b.emit(midir.Inst{Op: midir.OpBorrow, Dst: b.temp(idT), Args: []midir.LocalID{v}, Index: 1, Node: n})
		owned := b.emit(midir.Inst{Op: midir.OpShare, Dst: b.temp(idT), Args: []midir.LocalID{ref}, Node: n, Copy: true})
		b.emit(midir.Inst{Op: midir.OpNewCell, Dst: id, Args: []midir.LocalID{owned}, Node: n})
		b.unowned[id] = false
		return
	}
	if tt.Kind(idT) == sem.KRef && vNotRef {
		mut := 0
		if tt.RefMut(idT) {
			mut = 1
		}
		b.emit(midir.Inst{Op: midir.OpBorrow, Dst: id, Args: []midir.LocalID{v}, Index: mut, Node: n})
		return
	}
	if tt.Kind(idT) != sem.KRef && !vNotRef {
		b.emit(midir.Inst{Op: midir.OpCopy, Dst: id, Args: []midir.LocalID{b.retagTo(v, b.fn.Locals[v].Type, idT, n)}, Node: n, Share: b.shared[v]})
		return
	}
	if !b.guardedArm && b.wholeLocal(v) {
		b.emit(midir.Inst{Op: midir.OpMove, Dst: id, Args: []midir.LocalID{v}, Node: n})
		return
	}
	b.emit(midir.Inst{Op: midir.OpCopy, Dst: id, Args: []midir.LocalID{v}, Node: n, Share: b.shared[v], Guarded: b.guardedArm})
	if b.guardedArm && (!b.shared[v] || b.readsCell(v)) && !b.r.IsCopy(b.fn.Locals[v].Type) {
		b.guardMoves = append(b.guardMoves, guardMove{place: v, node: n, bind: id})
	}
}

type guardMove struct {
	place midir.LocalID
	node  syntax.NodeID
	bind  midir.LocalID
	// take empties the scrutinee place once the guard passed (takesScrutinee).
	take *hir.Pat
}

func (b *builder) bindGuarded(p *hir.Pat, v midir.LocalID, guarded bool) []guardMove {
	b.guardedArm = guarded
	b.bindPattern(p, v)
	b.guardedArm = false
	moves := b.guardMoves
	b.guardMoves = nil
	if b.takesScrutinee(p, v) {
		if !guarded {
			b.takeScrutinee(v, p, false)
		} else {
			moves = append(moves, guardMove{place: v, take: p})
		}
	}
	return moves
}
