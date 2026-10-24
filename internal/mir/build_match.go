package mir

import (
	"kigumi/internal/hir"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

func (b *builder) matchExpr(e *hir.Expr) LocalID {
	scrutinee := b.fullValue(func() LocalID { return b.expr(e.Args[0]) })
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
		b.term(Term{Op: TermBranch, Args: []LocalID{cond}, Targets: []BlockID{body, next}})
		b.cur = body
		b.pushScope()
		moves := b.bindGuarded(arm.Pat, scrutinee, arm.Guard != nil)
		if arm.Guard != nil {
			gv := b.fullValue(func() LocalID { return b.expr(arm.Guard) })
			run, fail := b.newBlockAt(), b.newBlockAt()
			b.term(Term{Op: TermBranch, Args: []LocalID{gv}, Targets: []BlockID{run, fail}})
			// A failed guard leaves the arm: its bindings die here too.
			b.cur = fail
			b.unwindScope(b.scopes[len(b.scopes)-1], false)
			b.jump(next)
			b.cur = run
			b.markGuardMoves(moves)
		}
		b.assignValue(result, b.armBody(arm.Body), e.Node)
		b.exitScope()
		b.jump(join)
		b.cur = next
	}
	b.emit(Inst{Op: OpPanic, Str: "no match arm matched", Node: e.Node, NoTrap: true})
	b.term(Term{Op: TermUnreachable})
	b.cur = join
	b.exitScope()
	return result
}

// bindPattern declares the bindings of a pattern that already matched.
func (b *builder) bindPattern(p *hir.Pat, v LocalID) {
	switch p.Kind {
	case hir.PatBind:
		id := b.declare(p.Ent)
		if b.fn.Locals[id].Cell {
			if b.shared[v] {
				v = b.emit(Inst{Op: OpCopy, Dst: b.temp(b.fn.Locals[v].Type), Args: []LocalID{v}, Node: p.Node, Share: true})
			}
			b.emit(Inst{Op: OpNewCell, Dst: id, Args: []LocalID{v}, Node: p.Node})
			return
		}
		b.bindLocal(id, v, p.Node)
		b.ownIfCounted(id)
	case hir.PatCtor:
		if p.IsType {
			if len(p.Subs) == 1 {
				b.bindPattern(p.Subs[0], b.emit(Inst{Op: OpUnbox, Dst: b.temp(p.Subs[0].Type), Args: []LocalID{v}, Node: p.Node}))
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
			b.bindPattern(s, b.emit(Inst{Op: OpPayload, Dst: b.temp(s.Type), Args: []LocalID{v}, Index: i, Node: s.Node}))
		}
	case hir.PatRecord:
		for _, f := range p.Fields {
			var val LocalID
			if b.takesField(p, f, v) {
				val = b.emit(Inst{Op: OpFieldMove, Dst: b.temp(f.Type), Args: []LocalID{v}, Index: f.Index, Node: f.Node})
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
func (b *builder) bindLocal(id, v LocalID, n syntax.NodeID) {
	tt := b.r.Types
	idT := b.fn.Locals[id].Type
	vNotRef := tt.Kind(b.fn.Locals[v].Type) != sem.KRef
	if _, ok := b.r.ScalarRefElem(idT); ok && vNotRef {
		// v's own type is the unwrapped payload (binding mode strips
		// the pattern's own type to the scrutinee), so it needs the same
		// OpBorrow retype + OpShare retain as an explicit `&mut expr` of a
		// place (build_expr.go's hir.Borrow) before OpNewCell can take it:
		// OpNewCell requires its argument's static type to already match id's.
		ref := b.emit(Inst{Op: OpBorrow, Dst: b.temp(idT), Args: []LocalID{v}, Index: 1, Node: n})
		owned := b.emit(Inst{Op: OpShare, Dst: b.temp(idT), Args: []LocalID{ref}, Node: n, Copy: true})
		b.emit(Inst{Op: OpNewCell, Dst: id, Args: []LocalID{owned}, Node: n})
		b.unowned[id] = false
		return
	}
	if tt.Kind(idT) == sem.KRef && vNotRef {
		mut := 0
		if tt.RefMut(idT) {
			mut = 1
		}
		b.emit(Inst{Op: OpBorrow, Dst: id, Args: []LocalID{v}, Index: mut, Node: n})
		return
	}
	if tt.Kind(idT) != sem.KRef && !vNotRef {
		b.emit(Inst{Op: OpCopy, Dst: id, Args: []LocalID{b.retagTo(v, b.fn.Locals[v].Type, idT, n)}, Node: n, Share: b.shared[v]})
		return
	}
	if !b.guardedArm && b.wholeLocal(v) {
		b.emit(Inst{Op: OpMove, Dst: id, Args: []LocalID{v}, Node: n})
		return
	}
	b.emit(Inst{Op: OpCopy, Dst: id, Args: []LocalID{v}, Node: n, Share: b.shared[v], Guarded: b.guardedArm})
	if b.guardedArm && (!b.shared[v] || b.readsCell(v)) && !b.r.IsCopy(b.fn.Locals[v].Type) {
		b.guardMoves = append(b.guardMoves, guardMove{place: v, node: n, bind: id})
	}
}

type guardMove struct {
	place LocalID
	node  syntax.NodeID
	bind  LocalID
	// take empties the scrutinee place once the guard passed (takesScrutinee).
	take *hir.Pat
}

func (b *builder) bindGuarded(p *hir.Pat, v LocalID, guarded bool) []guardMove {
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
