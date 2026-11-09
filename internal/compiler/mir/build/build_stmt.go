package build

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

func (b *builder) stmt(s *hir.Stmt) {
	if s.Kind == hir.DeferStmt {
		sc := b.scopes[len(b.scopes)-1]
		sc.defers = append(sc.defers, s)
		return
	}
	b.fullExpr(func() { b.fullStmt(s) })
}

func (b *builder) fullStmt(s *hir.Stmt) {
	switch s.Kind {
	case hir.LetStmt:
		b.letStmt(s)
	case hir.AssignStmt:
		b.assignStmt(s)
	case hir.DiscardStmt:
		b.discard(s)
	case hir.ExprStmt:
		if b.unparen(s.Expr).Kind == hir.Local {
			b.discard(&hir.Stmt{Kind: hir.DiscardStmt, Expr: b.unparen(s.Expr), Node: s.Node})
			return
		}
		if b.settles(s.Expr) {
			return
		}
		v := b.expr(s.Expr)
		b.emit(midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{v}, Node: s.Node})
	case hir.EvalStmt:
		b.expr(s.Expr)
	}
}

func (b *builder) letStmt(s *hir.Stmt) {
	if s.Else == nil && s.Pat.Kind == hir.PatWild && !s.Pat.Borrow && b.settles(s.Expr) {
		return
	}
	var v midir.LocalID
	if s.Pat.Borrow {
		v = b.expr(s.Expr)
		b.ownTemp(v)
	} else {
		v = b.exprMoved(s.Expr)
	}
	if s.Else == nil {
		b.bindPattern(s.Pat, v)
		return
	}
	okBlock, elseBlock := b.newBlockAt(), b.newBlockAt()
	cond := b.testPattern(s.Pat, v)
	b.term(midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{cond}, Targets: []midir.BlockID{okBlock, elseBlock}})
	b.cur = elseBlock
	b.blockExpr(s.Else)
	b.term(midir.Term{Op: midir.TermUnreachable})
	b.cur = okBlock
	b.bindPattern(s.Pat, v)
}

func (b *builder) isParen(e *hir.Expr) bool {
	return e.Kind == hir.Wrap && len(e.Args) == 1 && b.r.Tree(b.fn.File).Kind(e.Node) == syntax.Paren
}

func (b *builder) isSpread(e *hir.Expr) bool {
	return e.Kind == hir.Wrap && len(e.Args) == 1 && b.r.Tree(b.fn.File).Kind(e.Node) == syntax.Spread
}

func (b *builder) unparen(e *hir.Expr) *hir.Expr {
	for b.isParen(e) {
		e = e.Args[0]
	}
	return e
}

// exprMoved lowers an expression whose value leaves the current scope: a
// named local is copied or moved into a temporary before any coercion
// wraps it, so the scope's drop of the local stays balanced.
func (b *builder) exprMoved(e *hir.Expr) midir.LocalID {
	if b.isParen(e) || b.isSpread(e) {
		return b.coerce(e.Coercion, e.Node, b.exprMoved(e.Args[0]))
	}
	if b.selfFieldMove(e) {
		base := b.expr(e.Args[0])
		v := b.emit(midir.Inst{Op: midir.OpFieldMove, Dst: b.temp(e.Type), Args: []midir.LocalID{base}, Index: e.Index, Node: e.Node})
		return b.coerce(e.Coercion, e.Node, v)
	}
	v := b.transfer(e, b.exprRaw(e))
	if e.Kind == hir.Fallback && e.HitLift {
		return v
	}
	return b.coerce(e.Coercion, e.Node, v)
}

// transfer moves a move-only source local or copies a structural value,
// seeing through a wrap/unwrap to its receiver and retagging the result back.
func (b *builder) transfer(outer *hir.Expr, v midir.LocalID) midir.LocalID {
	src := outer
	for src.Kind == hir.DefinedWrap || src.Kind == hir.DefinedUnwrap {
		src = src.Args[0]
	}
	wrapped := src != outer
	retag := func(out midir.LocalID) midir.LocalID {
		if !wrapped {
			return out
		}
		return b.emit(midir.Inst{Op: midir.OpAlias, Dst: b.temp(outer.Type), Args: []midir.LocalID{out}, Node: outer.Node})
	}
	if wrapped && b.derefsRef(src.Type, outer.Type) {
		// What leaves is a copy of the referent, so the reference itself is neither copied nor moved.
		return b.emit(midir.Inst{Op: midir.OpShare, Dst: b.temp(outer.Type), Args: []midir.LocalID{retag(v)}, Copy: true, Node: outer.Node})
	}
	if src.Kind != hir.Local {
		return retag(v)
	}
	id, isLocal := b.locals[src.Ent]
	if src.Ent == 0 || !isLocal {
		return retag(v)
	}
	// A named local is dropped when its scope unwinds, so what leaves the
	// scope travels in a temporary: a copy of a Copy value, a move otherwise.
	t := src.Type
	switch {
	case b.r.IsCopy(t) && b.fn.Locals[id].Cell:
		// v is an OpCellGet read; RC would alias an Ent-less OpCopy
		// source, drifting with the cell's later in-place field writes.
		return retag(b.emit(midir.Inst{Op: midir.OpShare, Dst: b.temp(t), Args: []midir.LocalID{v}, Copy: true, Node: src.Node}))
	case b.r.IsCopy(t):
		return retag(b.emit(midir.Inst{Op: midir.OpCopy, Dst: b.temp(t), Args: []midir.LocalID{v}, Node: src.Node}))
	}
	return retag(b.emit(midir.Inst{Op: midir.OpMove, Dst: b.temp(t), Args: []midir.LocalID{v}, Node: src.Node}))
}

// discard lowers `_ = value`. Discarding a binding changes nothing unless
// it is move-only: the binding keeps a borrowed or Copy value and drops it
// at scope exit, while a move-only value is moved out and dropped here.
func (b *builder) discard(s *hir.Stmt) {
	rhs := s.Expr
	if b.settles(rhs) {
		return
	}
	if l, ok := b.locals[rhs.Ent]; ok && rhs.Kind == hir.Local {
		lt := b.fn.Locals[l].Type
		if b.fn.Locals[l].Borrowed || b.r.IsCopy(lt) {
			return
		}
	}
	v := b.exprMoved(rhs)
	b.emit(midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{v}, Node: s.Node})
}

func (b *builder) assignStmt(s *hir.Stmt) {
	p := b.resolvePlace(s.Place)
	if s.Place.Kind == hir.PlaceIndex {
		p.idx = b.snapshot(p.idx, s.Place.Idx, s.Place.Idx.Node, s.Expr)
	}
	if s.Compound {
		cur := b.snapshot(b.loadPlace(p), nil, s.Place.Node, s.Expr)
		r := b.expr(s.Expr)
		t := s.Place.Type
		if elem, ok := b.r.MutRefElem(t); ok {
			t = elem
		}
		var out midir.LocalID
		if s.OpEnt != 0 {
			out = b.emit(midir.Inst{Op: midir.OpCall, Dst: b.temp(t), Ent: s.OpEnt, Args: []midir.LocalID{cur, r}, Node: s.Node})
		} else {
			out = b.emit(midir.Inst{Op: midir.OpBinary, Dst: b.temp(t), Str: s.OpText, Args: []midir.LocalID{cur, r}, Node: s.Node})
		}
		b.storePlace(p, out)
		return
	}
	b.storePlace(p, b.exprMoved(s.Expr))
}

func (b *builder) selfFieldMove(e *hir.Expr) bool {
	if e.Kind != hir.Field || len(e.Args) != 1 || e.Args[0].Kind != hir.Local || b.fn.Ent == 0 {
		return false
	}
	info := b.r.Fn(b.fn.Ent)
	return !b.r.IsCopy(e.Type) && info.Recv == sem.RecvMove && e.Args[0].Ent == info.SelfParam
}
