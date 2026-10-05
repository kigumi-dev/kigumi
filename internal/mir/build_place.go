package mir

import (
	"kigumi/internal/diag"
	"kigumi/internal/hir"
	"kigumi/internal/sem"
)

// place is an assignment target whose operands have already been evaluated.
// Resolving once is what makes `arr[next()] += 1` call next() a single time and
// what orders base and index before the right-hand side.
type place struct {
	src  *hir.Place
	id   LocalID
	base LocalID
	idx  LocalID
	// wb marks a store that returns a receiver cell's content to the place
	// the call borrowed it from, not a write the source wrote.
	wb bool
}

func (b *builder) resolvePlace(pl *hir.Place) place {
	switch pl.Kind {
	case hir.PlaceField:
		return place{src: pl, base: b.expr(pl.Base)}
	case hir.PlaceIndex:
		base := b.expr(pl.Base)
		idx := b.expr(pl.Idx)
		return place{src: pl, base: base, idx: idx}
	case hir.PlaceGlobal:
		return place{src: pl}
	}
	return place{src: pl, id: b.local(pl.Ent)}
}

func (b *builder) loadPlace(p place) LocalID {
	switch p.src.Kind {
	case hir.PlaceField:
		return b.emit(Inst{Op: OpField, Dst: b.temp(p.src.Type), Args: []LocalID{p.base}, Index: p.src.Index, Node: p.src.Node})
	case hir.PlaceIndex:
		return b.emit(Inst{Op: OpIndex, Dst: b.temp(p.src.Type), Args: []LocalID{p.base, p.idx}, Node: p.src.Node})
	case hir.PlaceGlobal:
		return b.emit(Inst{Op: OpGlobalGet, Dst: b.temp(p.src.Type), Ent: p.src.Ent, Node: p.src.Node})
	}
	return b.expr(p.src.Expr)
}

// storePlace overwrites a place, reinitializing it: a field or
// element releases on the store itself, so a second drop would double-free;
// a named local is dropped through the local, not a copy, so the ownership
// pass sees a drop, not a read; a borrow rebinds the reference and owns
// nothing to drop.
func (b *builder) storePlace(p place, v LocalID) {
	switch p.src.Kind {
	case hir.PlaceField:
		b.emit(Inst{Op: OpSetField, Args: []LocalID{p.base, v}, Index: p.src.Index, Node: p.src.Node})
	case hir.PlaceIndex:
		b.emit(Inst{Op: OpSetIndex, Args: []LocalID{p.base, p.idx, v}, Node: p.src.Node})
	case hir.PlaceGlobal:
		b.emit(Inst{Op: OpGlobalSet, Args: []LocalID{v}, Ent: p.src.Ent, Node: p.src.Node})
	default:
		if _, ok := b.r.ScalarRefElem(b.fn.Locals[p.id].Type); b.fn.Locals[p.id].Cell || ok {
			b.emit(Inst{Op: OpCellSet, Args: []LocalID{p.id, v}, Node: p.src.Node})
			return
		}
		if b.fn.Locals[p.id].Borrowed {
			b.rejectBorrowedReassign(p)
			return
		}
		if b.r.Types.Kind(b.fn.Locals[p.id].Type) != sem.KRef {
			b.emit(Inst{Op: OpDrop, Args: []LocalID{p.id}, Scoped: p.wb, Node: p.src.Node})
		}
		b.emit(Inst{Op: OpCopy, Dst: p.id, Args: []LocalID{v}, Node: p.src.Node})
	}
}

// A lent binding with no cell of its own has nothing to carry the new value back to its owner.
func (b *builder) rejectBorrowedReassign(p place) {
	t := b.r.Tree(b.fn.File)
	msg := "`" + b.fn.Locals[p.id].Name + "` is lent to this body and cannot be reassigned as a whole"
	b.fn.Diags = append(b.fn.Diags, diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(p.src.Node)), Code: "borrowed-self-reassign", Msg: msg})
}
