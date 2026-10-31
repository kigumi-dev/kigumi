package mir

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/lang/diag"
)

// place is an assignment target whose operands have already been evaluated.
// Resolving once is what makes `arr[next()] += 1` call next() a single time and
// what orders base and index before the right-hand side.
type place struct {
	src  *hir.Place
	id   midir.LocalID
	base midir.LocalID
	idx  midir.LocalID
	// wb marks a store that returns a receiver cell's content to the place
	// the call borrowed it from, not a write the source wrote.
	wb bool
}

func (b *builder) resolvePlace(pl *hir.Place) place {
	switch pl.Kind {
	case hir.PlaceField:
		return place{src: pl, base: b.storeBase(pl.Base)}
	case hir.PlaceIndex:
		base := b.storeBase(pl.Base)
		idx := b.expr(pl.Idx)
		return place{src: pl, base: base, idx: idx}
	case hir.PlaceGlobal:
		return place{src: pl}
	}
	return place{src: pl, id: b.local(pl.Ent)}
}

// storeBase projects a stored-through base from its root, skipping the defined-type layers at every step, since a retagged layer would read a copy.
func (b *builder) storeBase(e *hir.Expr) midir.LocalID {
	e = definedBase(e)
	if len(e.Coerce) > 0 {
		return b.expr(e)
	}
	switch e.Kind {
	case hir.Field:
		base := b.storeBase(e.Args[0])
		return b.emit(midir.Inst{Op: midir.OpField, Dst: b.temp(e.Type), Args: []midir.LocalID{base}, Index: e.Index, Node: e.Node})
	case hir.Index:
		base := b.storeBase(e.Args[0])
		idx := b.expr(e.Args[1])
		return b.emit(midir.Inst{Op: midir.OpIndex, Dst: b.temp(e.Type), Args: []midir.LocalID{base, idx}, Node: e.Node})
	}
	return b.expr(e)
}

func (b *builder) loadPlace(p place) midir.LocalID {
	switch p.src.Kind {
	case hir.PlaceField:
		return b.emit(midir.Inst{Op: midir.OpField, Dst: b.temp(p.src.Type), Args: []midir.LocalID{p.base}, Index: p.src.Index, Node: p.src.Node})
	case hir.PlaceIndex:
		return b.emit(midir.Inst{Op: midir.OpIndex, Dst: b.temp(p.src.Type), Args: []midir.LocalID{p.base, p.idx}, Node: p.src.Node})
	case hir.PlaceGlobal:
		return b.emit(midir.Inst{Op: midir.OpGlobalGet, Dst: b.temp(p.src.Type), Ent: p.src.Ent, Node: p.src.Node})
	}
	return b.expr(p.src.Expr)
}

// storePlace overwrites a place, reinitializing it: a field or
// element releases on the store itself, so a second drop would double-free;
// a named local is dropped through the local, not a copy, so the ownership
// pass sees a drop, not a read; a borrow rebinds the reference and owns
// nothing to drop.
func (b *builder) storePlace(p place, v midir.LocalID) {
	switch p.src.Kind {
	case hir.PlaceField:
		b.emit(midir.Inst{Op: midir.OpSetField, Args: []midir.LocalID{p.base, v}, Index: p.src.Index, Node: p.src.Node})
	case hir.PlaceIndex:
		b.emit(midir.Inst{Op: midir.OpSetIndex, Args: []midir.LocalID{p.base, p.idx, v}, Node: p.src.Node})
	case hir.PlaceGlobal:
		b.emit(midir.Inst{Op: midir.OpGlobalSet, Args: []midir.LocalID{v}, Ent: p.src.Ent, Node: p.src.Node})
	default:
		if b.cellLocal(p.id) {
			b.emit(midir.Inst{Op: midir.OpCellSet, Args: []midir.LocalID{p.id, v}, Node: p.src.Node})
			return
		}
		if b.fn.Locals[p.id].Borrowed {
			b.rejectBorrowedReassign(p)
			return
		}
		if b.r.Types.Kind(b.fn.Locals[p.id].Type) != sem.KRef {
			b.emit(midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{p.id}, Scoped: p.wb, Node: p.src.Node})
		}
		b.emit(midir.Inst{Op: midir.OpCopy, Dst: p.id, Args: []midir.LocalID{v}, Node: p.src.Node})
	}
}

// A lent binding with no cell of its own has nothing to carry the new value back to its owner.
func (b *builder) rejectBorrowedReassign(p place) {
	t := b.r.Tree(b.fn.File)
	msg := "`" + b.fn.Locals[p.id].Name + "` is lent to this body and cannot be reassigned as a whole"
	b.fn.Diags = append(b.fn.Diags, diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(p.src.Node)), Code: "borrowed-self-reassign", Msg: msg})
}
