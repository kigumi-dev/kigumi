package mir

import (
	"kigumi/internal/hir"
	"kigumi/internal/sem"
)

// ownTemp hands a scrutinee temporary to the innermost scope, which must
// open after the temporary is assigned: pattern bindings copy out of it,
// so it is released with that scope on every exit.
func (b *builder) ownTemp(v LocalID) {
	if b.fn.Locals[v].Ent == 0 {
		b.own(v)
		b.shared[v] = true
	}
}

// isExpr lowers `v is pat`; an `is` may sit behind `&&`, so a temporary scrutinee dies at the join
// after the bindings copy out of it, unless kept says the enclosing scope already owns it.
func (b *builder) isExpr(p *hir.Pat, v LocalID, kept bool) LocalID {
	cond := b.testPattern(p, v)
	yes, join := b.newBlockAt(), b.newBlockAt()
	b.term(Term{Op: TermBranch, Args: []LocalID{cond}, Targets: []BlockID{yes, join}})
	b.cur = yes
	if b.fn.Locals[v].Ent == 0 {
		b.shared[v] = true
	}
	if p != nil && b.condIs[p] {
		delete(b.condIs, p)
		b.isMoves = append(b.isMoves, b.bindGuarded(p, v, true)...)
	} else {
		b.bindPattern(p, v)
	}
	b.jump(join)
	b.cur = join
	if b.fn.Locals[v].Ent == 0 && !kept {
		b.emit(Inst{Op: OpDrop, Args: []LocalID{v}, Node: p.Node})
	}
	return cond
}

func (b *builder) cond(e *hir.Expr) (LocalID, []guardMove) {
	pats := map[*hir.Pat]bool{}
	condTests(e, pats)
	if len(pats) == 0 {
		return b.expr(e), nil
	}
	savedIs, saved := b.condIs, b.isMoves
	b.condIs, b.isMoves = pats, nil
	c := b.expr(e)
	moves := b.isMoves
	b.condIs, b.isMoves = savedIs, saved
	return c, moves
}

// valueAnd opens a scope for an `&&` chain outside a condition that drops its `is` bindings and the temporaries they borrow where the chain ends;
// a chain nested on a right side gets its own scope when the enclosing chain does not read its bindings.
func (b *builder) valueAnd(e *hir.Expr) LocalID {
	pats := map[*hir.Pat]bool{}
	condTests(e, pats)
	if len(pats) == 0 {
		return b.shortAnd(e)
	}
	nested := false
	for p := range pats {
		if b.condIs[p] {
			return b.shortAnd(e)
		}
		nested = nested || b.valueIs[p]
	}
	if nested && (b.shortRHS == 0 || b.chainReads(e, pats)) {
		return b.shortAnd(e)
	}
	savedIs, savedShort, savedChain := b.valueIs, b.shortRHS, b.valueChain
	b.valueIs, b.shortRHS, b.valueChain = pats, 0, e
	b.pushScope()
	c := b.shortAnd(e)
	chain := b.scopes[len(b.scopes)-1]
	b.scopes = b.scopes[:len(b.scopes)-1]
	b.unwindScope(chain, false)
	b.valueIs, b.shortRHS, b.valueChain = savedIs, savedShort, savedChain
	return c
}

func (b *builder) chainReads(inner *hir.Expr, pats map[*hir.Pat]bool) bool {
	ents := map[sem.EntityID]bool{}
	for p := range pats {
		patBinds(p, ents)
	}
	return mentions(b.valueChain, inner, ents)
}

func condTests(e *hir.Expr, into map[*hir.Pat]bool) {
	switch e.Kind {
	case hir.Is:
		if e.Pat != nil {
			into[e.Pat] = true
		}
	case hir.And:
		condTests(e.Args[0], into)
		condTests(e.Args[1], into)
	case hir.Wrap:
		condTests(e.Args[0], into)
	case hir.Unary:
		if inner := notOperand(e); inner != nil {
			if pos := notOperand(inner); pos != nil {
				condTests(pos, into)
			}
		}
	}
}

func notOperand(e *hir.Expr) *hir.Expr {
	for e.Kind == hir.Wrap {
		e = e.Args[0]
	}
	if e.Kind != hir.Unary || e.Name != "!" || e.Ent != 0 {
		return nil
	}
	return e.Args[0]
}
