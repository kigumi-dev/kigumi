package mir

import (
	"kigumi/internal/hir"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// baseLocal peels defined-type wrap/unwrap layers (same storage as their
// receiver) down to the hir.Local underneath, if any.
func baseLocal(e *hir.Expr) (*hir.Expr, bool) {
	for e.Kind == hir.DefinedWrap || e.Kind == hir.DefinedUnwrap {
		e = e.Args[0]
	}
	if e.Kind != hir.Local {
		return nil, false
	}
	return e, true
}

func (b *builder) borrowExpr(e *hir.Expr, n syntax.NodeID, t sem.TypeID) LocalID {
	// A scalar `&mut` of a bare local/parameter aliases that local's own
	// cell (sem flags it EfAddrTaken) instead of boxing a disconnected copy.
	if e.Mut {
		if _, ok := b.r.ScalarRefElem(t); ok {
			if base, ok := baseLocal(e.Args[0]); ok {
				if id := b.local(base.Ent); b.fn.Locals[id].Cell {
					ref := b.emit(Inst{Op: OpBorrow, Dst: b.temp(b.r.Types.Ref(base.Type, true)), Args: []LocalID{id}, Index: 1, Node: n})
					if base != e.Args[0] {
						ref = b.emit(Inst{Op: OpAlias, Dst: b.temp(t), Args: []LocalID{ref}, Node: n})
					}
					return ref
				}
			}
		}
	}
	v := b.expr(e.Args[0])
	mut := 0
	if e.Mut {
		mut = 1
	}
	ref := b.emit(Inst{Op: OpBorrow, Dst: b.temp(t), Args: []LocalID{v}, Index: mut, Node: n})
	// A scalar `&mut` must be stored through by replacing its referent, so
	// it is boxed; the box takes its own reference since a borrow owns nothing.
	if e.Mut {
		if _, ok := b.r.ScalarRefElem(t); ok {
			owned := b.emit(Inst{Op: OpShare, Dst: b.temp(t), Args: []LocalID{ref}, Node: n, Copy: true})
			return b.emit(Inst{Op: OpNewCell, Dst: b.temp(t), Args: []LocalID{owned}, Node: n})
		}
	}
	return ref
}
