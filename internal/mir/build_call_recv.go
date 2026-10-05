package mir

import (
	"kigumi/internal/hir"
	"kigumi/internal/sem"
)

// Assigning self as a whole lands back in the caller's storage, but only
// when sync; async (sync false) cannot write back.
func (b *builder) recvCell(base *hir.Expr, sync bool) LocalID {
	n, inner := base.Node, peelDefined(base)
	rt := b.r.Types.Ref(base.Type, true)
	p, hasPlace := b.recvPlace(base, inner, sync)
	var v LocalID
	if hasPlace {
		v = b.retagTo(b.loadPlace(p), inner.Type, base.Type, n)
	} else {
		v = b.expr(base)
	}
	ref := b.emit(Inst{Op: OpBorrow, Dst: b.temp(rt), Args: []LocalID{v}, Index: 1, Node: n})
	owned := b.emit(Inst{Op: OpShare, Dst: b.temp(rt), Args: []LocalID{ref}, Node: n})
	cell := b.emit(Inst{Op: OpNewCell, Dst: b.temp(rt), Args: []LocalID{owned}, Node: n, Scoped: sync})
	if sync {
		b.argFixups = append(b.argFixups, argFixup{recv: &p, elemType: base.Type, retag: inner.Type, cell: cell, node: n})
	}
	return cell
}

func peelDefined(e *hir.Expr) *hir.Expr {
	for e.Kind == hir.DefinedWrap || e.Kind == hir.DefinedUnwrap {
		e = e.Args[0]
	}
	return e
}

// Evaluates base and index once; a receiver with no place gets no write-back.
func (b *builder) recvPlace(base, inner *hir.Expr, sync bool) (place, bool) {
	if !sync || len(base.Coerce) > 0 || len(inner.Coerce) > 0 || b.r.Types.Kind(inner.Type) == sem.KRef {
		return place{}, false
	}
	pl := &hir.Place{Node: inner.Node, Type: inner.Type}
	switch inner.Kind {
	case hir.Local:
		id := b.local(inner.Ent)
		if b.fn.Locals[id].Borrowed && !b.fn.Locals[id].Cell {
			return place{}, false
		}
		pl.Kind, pl.Ent, pl.Expr = hir.PlaceLocal, inner.Ent, inner
	case hir.Field:
		pl.Kind, pl.Base, pl.Index = hir.PlaceField, inner.Args[0], inner.Index
	case hir.Index:
		pl.Kind, pl.Base, pl.Idx = hir.PlaceIndex, inner.Args[0], inner.Args[1]
	case hir.Global:
		pl.Kind, pl.Ent = hir.PlaceGlobal, inner.Ent
	default:
		return place{}, false
	}
	p := b.resolvePlace(pl)
	p.wb = true
	return p, true
}
