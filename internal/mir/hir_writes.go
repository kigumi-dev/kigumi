package mir

import (
	"kigumi/internal/hir"
	"kigumi/internal/sem"
)

// writesLocal reports whether evaluating e can store into local ent; a closure body is skipped
// because a local a closure writes lives in a cell, whose reads copy.
func writesLocal(r *sem.Result, e *hir.Expr, ent sem.EntityID) bool {
	return localWrites{r: r, ent: ent}.expr(e)
}

// moves also counts a by-value use of an owned ent as a write, since moving it out ends the value too;
// path limits the writes to places overlapping that field path of ent, or with whole to places containing it.
type localWrites struct {
	r     *sem.Result
	ent   sem.EntityID
	whole bool
	moves bool
	// deep keeps a move from inside the receiver a write under whole, since a resource's lent view would see the hole.
	deep bool
	// lends marks ent as a `&mut` reference, whose plain use may replace the whole referent.
	lends bool
	path  []int
	// steps is the receiver's path with its elements kept, which path loses past an index.
	steps []int
}

func (w localWrites) expr(e *hir.Expr) bool {
	r, ent := w.r, w.ent
	if e == nil {
		return false
	}
	switch e.Kind {
	case hir.Lambda:
		return false
	case hir.Local:
		return e.Ent == ent && (w.lends || w.moves && !r.IsCopy(e.Type) && r.Types.Kind(e.Type) != sem.KRef)
	case hir.Borrow:
		if e.Mut && w.hits(e.Args[0]) {
			return true
		}
	case hir.FieldMove:
		if !w.whole && w.hits(e.Args[0]) || w.whole && w.movesOut(e) {
			return true
		}
	case hir.MethodCall:
		if len(e.Args) > 0 && e.Ent != 0 && r.Entity(e.Ent).Kind == sem.EntFn {
			switch r.Fn(e.Ent).Recv {
			case sem.RecvMove:
				if w.movesOut(e.Args[0]) {
					return true
				}
			case sem.RecvMut:
				if w.hits(e.Args[0]) && (!w.whole || r.ReplacesSelf(e.Ent) || w.above(e.Args[0])) {
					return true
				}
			}
		}
	}
	for i, a := range e.Args {
		if i == 0 && w.moves && placeBase[e.Kind] && a.Kind == hir.Local {
			continue
		}
		if w.expr(a) {
			return true
		}
	}
	for _, a := range e.CtxArgs {
		if w.expr(a) {
			return true
		}
	}
	for _, en := range e.Entries {
		if w.expr(en.Value) {
			return true
		}
	}
	for _, arm := range e.Arms {
		if w.expr(arm.Guard) || w.expr(arm.Body) {
			return true
		}
	}
	return w.expr(e.Cond) || w.expr(e.Else) || w.expr(e.Guard) || w.block(e.Then) || w.block(e.Block)
}

var placeBase = map[hir.ExprKind]bool{hir.Field: true, hir.OptField: true, hir.Index: true, hir.Borrow: true, hir.MethodCall: true, hir.MethodValue: true}

func (w localWrites) block(blk *hir.Block) bool {
	if blk == nil {
		return false
	}
	for _, s := range blk.Stmts {
		if w.stmt(s) {
			return true
		}
	}
	return w.expr(blk.Tail)
}

func (w localWrites) stmt(s *hir.Stmt) bool {
	if s == nil {
		return false
	}
	if pl := s.Place; pl != nil {
		switch pl.Kind {
		case hir.PlaceLocal:
			if pl.Ent == w.ent {
				return true
			}
		case hir.PlaceField, hir.PlaceIndex:
			if w.hitsPlace(pl) || pl.Base.Kind != hir.Local && w.expr(pl.Base) || w.expr(pl.Idx) {
				return true
			}
		}
	}
	return w.expr(s.Expr) || w.block(s.Else) || w.stmt(s.Body) || w.block(s.Block)
}

// rootedAt reports whether e names local ent or a field or element reached from it.
func rootedAt(e *hir.Expr, ent sem.EntityID) bool {
	for e != nil {
		switch e.Kind {
		case hir.Local:
			return e.Ent == ent
		case hir.Field, hir.FieldMove, hir.Index, hir.Wrap, hir.DefinedWrap, hir.DefinedUnwrap:
			e = e.Args[0]
		default:
			return false
		}
	}
	return false
}

// mutBorrowsReceiver: a later `&mut` of the receiver's own local is an exclusivity conflict the borrow checker reports, not a write to read past.
func mutBorrowsReceiver(recv *hir.Expr, later []*hir.Expr) bool {
	if recv == nil || recv.Kind != hir.Local {
		return false
	}
	for _, e := range later {
		if e != nil && e.Kind == hir.Borrow && e.Mut && rootedAt(e.Args[0], recv.Ent) {
			return true
		}
	}
	return false
}
