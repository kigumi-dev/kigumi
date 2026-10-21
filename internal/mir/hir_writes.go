package mir

import (
	"kigumi/internal/hir"
	"kigumi/internal/sem"
)

// writesLocal reports whether evaluating e can store into local ent; a closure body is skipped
// because a local a closure writes lives in a cell, whose reads copy.
func writesLocal(r *sem.Result, e *hir.Expr, ent sem.EntityID) bool {
	if e == nil {
		return false
	}
	switch e.Kind {
	case hir.Lambda:
		return false
	case hir.Borrow:
		if e.Mut && rootedAt(e.Args[0], ent) {
			return true
		}
	case hir.FieldMove:
		if rootedAt(e.Args[0], ent) {
			return true
		}
	case hir.MethodCall:
		if len(e.Args) > 0 && e.Ent != 0 && r.Entity(e.Ent).Kind == sem.EntFn {
			if recv := r.Fn(e.Ent).Recv; (recv == sem.RecvMut || recv == sem.RecvMove) && rootedAt(e.Args[0], ent) {
				return true
			}
		}
	}
	for _, a := range e.Args {
		if writesLocal(r, a, ent) {
			return true
		}
	}
	for _, a := range e.CtxArgs {
		if writesLocal(r, a, ent) {
			return true
		}
	}
	for _, en := range e.Entries {
		if writesLocal(r, en.Value, ent) {
			return true
		}
	}
	for _, arm := range e.Arms {
		if writesLocal(r, arm.Guard, ent) || writesLocal(r, arm.Body, ent) {
			return true
		}
	}
	return writesLocal(r, e.Cond, ent) || writesLocal(r, e.Else, ent) || writesLocal(r, e.Guard, ent) ||
		blockWrites(r, e.Then, ent) || blockWrites(r, e.Block, ent)
}

func blockWrites(r *sem.Result, blk *hir.Block, ent sem.EntityID) bool {
	if blk == nil {
		return false
	}
	for _, s := range blk.Stmts {
		if stmtWrites(r, s, ent) {
			return true
		}
	}
	return writesLocal(r, blk.Tail, ent)
}

func stmtWrites(r *sem.Result, s *hir.Stmt, ent sem.EntityID) bool {
	if s == nil {
		return false
	}
	if pl := s.Place; pl != nil {
		switch pl.Kind {
		case hir.PlaceLocal:
			if pl.Ent == ent {
				return true
			}
		case hir.PlaceField, hir.PlaceIndex:
			if rootedAt(pl.Base, ent) || writesLocal(r, pl.Base, ent) || writesLocal(r, pl.Idx, ent) {
				return true
			}
		}
	}
	return writesLocal(r, s.Expr, ent) || blockWrites(r, s.Else, ent) || stmtWrites(r, s.Body, ent) || blockWrites(r, s.Block, ent)
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
