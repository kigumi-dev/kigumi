package mir

import (
	"kigumi/internal/compiler/hir"
	"kigumi/internal/compiler/sem"
)

// fullState numbers the full expression being lowered; held: one of its temporaries may hold a resource.
type fullState struct {
	id, next int32
	held     bool
	memo     map[sem.TypeID]bool
}

func (s *fullState) note(r *sem.Result, t sem.TypeID) {
	if s.id == 0 || s.held {
		return
	}
	held, ok := s.memo[t]
	if !ok {
		if s.memo == nil {
			s.memo = map[sem.TypeID]bool{}
		}
		held = r.HoldsResource(t)
		s.memo[t] = held
	}
	s.held = held
}

// fullExpr marks the last instruction of eval, where the plan releases its resource temporaries.
func (b *builder) fullExpr(eval func()) {
	savedID, savedHeld := b.full.id, b.full.held
	b.full.next++
	b.full.id, b.full.held = b.full.next, false
	eval()
	if b.full.held {
		b.markFullEnd(b.full.id)
	}
	b.full.id, b.full.held = savedID, savedHeld
}

// A block with nothing in it yet holds no temporary the mark could release.
func (b *builder) markFullEnd(id int32) {
	blk := b.block()
	n := len(blk.Insts)
	switch {
	case n == 0 || blk.Term.Op != TermNone:
	case blk.Insts[n-1].FullEnd == 0:
		blk.Insts[n-1].FullEnd = id
	default:
		b.emit(Inst{Op: OpNop, FullEnd: id})
	}
}

func (b *builder) fullValue(eval func() LocalID) LocalID {
	var v LocalID
	b.fullExpr(func() { v = eval() })
	return v
}

// armBody: a braced body already ends its own full expressions.
func (b *builder) armBody(e *hir.Expr) LocalID {
	if e.Kind == hir.BlockExpr {
		return b.exprMoved(e)
	}
	return b.fullValue(func() LocalID { return b.exprMoved(e) })
}
