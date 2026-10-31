package mir

import (
	"kigumi/internal/compiler/hir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// Only a `&mut T` local ever holds a write-back cell over its referent rather than the value itself.
func (b *builder) cellLocal(id LocalID) bool {
	l := &b.fn.Locals[id]
	if l.Cell {
		return true
	}
	if _, ok := b.r.ScalarRefElem(l.Type); ok {
		return true
	}
	_, mutRef := b.r.MutRefElem(l.Type)
	return mutRef && l.Ent != 0 && b.r.RefCellParam(l.Ent)
}

// A `&mut` binding already lent as a cell is handed on as is: wrapping it again would send the new value to a temporary.
func (b *builder) refCellBase(e *hir.Expr) (LocalID, bool) {
	if len(e.Coerce) > 0 {
		return 0, false
	}
	inner := peelNoOp(e)
	if inner.Kind != hir.Local || len(inner.Coerce) > 0 {
		return 0, false
	}
	id := b.local(inner.Ent)
	if _, scalar := b.r.ScalarRefElem(b.fn.Locals[id].Type); scalar || !b.cellLocal(id) {
		return 0, false
	}
	return id, b.r.Types.Kind(b.fn.Locals[id].Type) == sem.KRef
}

// An unknown callee is assumed to want a cell, its body being unreadable here; a bodiless std primitive or foreign function does not.
func (b *builder) wantsRefCell(ent sem.EntityID, idx int) bool {
	if ent == 0 {
		return true
	}
	var params []sem.EntityID
	switch b.r.Entity(ent).Kind {
	case sem.EntClosure:
		params = b.r.Closure(ent).Params
	case sem.EntFn:
		info := b.r.Fn(ent)
		if info.Abi != "" || info.Naked || b.p.IsStdPrimitive(ent) {
			return false
		}
		params = info.Params
	default:
		return false
	}
	if idx < 0 || idx >= len(params) {
		return false
	}
	return b.r.RefCellParam(params[idx])
}

// applyArgFixups stores the cell back into the place afterwards.
func (b *builder) mutRefArg(a *hir.Expr, idx int, ent sem.EntityID, sync bool) (LocalID, bool) {
	if !sync || !b.wantsRefCell(ent, idx) {
		return 0, false
	}
	if _, scalar := b.r.ScalarRefElem(a.Type); scalar {
		return 0, false
	}
	borrow := peelNoOp(a)
	if borrow.Kind != hir.Borrow || !borrow.Mut {
		return b.lentRefArg(a)
	}
	base, n := borrow.Args[0], borrow.Node
	inner := peelNoOp(base)
	p, hasPlace := b.exprPlace(base, inner, sync)
	var v LocalID
	if hasPlace {
		v = b.retagTo(b.loadPlace(p), inner.Type, base.Type, n)
	} else {
		v = b.expr(base)
	}
	cell := b.lendCell(v, borrow.Type, n, sync)
	b.argFixups = append(b.argFixups, argFixup{recv: &p, elemType: base.Type, retag: inner.Type, cell: cell, node: n, depth: len(b.scopes)})
	return b.coerce(a.Coercion, n, cell), true
}

// Any other `&mut` value names no place of this frame's, so sem rejects it (E1503); the cell here only keeps the callee's store well formed.
func (b *builder) lentRefArg(a *hir.Expr) (LocalID, bool) {
	elem, ok := b.r.MutRefElem(a.Type)
	if !ok {
		return 0, false
	}
	raw, lend := *a, lendsExistential(a.Coercion)
	if lend {
		raw.Coercion = hir.Coercion{}
	}
	if _, cell := b.refCellBase(&raw); cell {
		return 0, false
	}
	cell := b.cellOver(b.expr(&raw), a.Type, a.Node, true)
	b.argFixups = append(b.argFixups, argFixup{recv: &place{}, elemType: elem, retag: elem, cell: cell, node: a.Node, depth: len(b.scopes)})
	if lend {
		return b.coerce(a.Coercion, a.Node, cell), true
	}
	return cell, true
}

func lendsExistential(co hir.Coercion) bool {
	for _, s := range co.Coerce {
		if s.Kind == sem.CoLendExistential {
			return true
		}
	}
	return false
}

func (b *builder) lendCell(v LocalID, t sem.TypeID, n syntax.NodeID, scoped bool) LocalID {
	return b.cellOver(b.emit(Inst{Op: OpBorrow, Dst: b.temp(t), Args: []LocalID{v}, Index: 1, Node: n}), t, n, scoped)
}

// The cell takes its own refcount; the borrow passed in owns none.
func (b *builder) cellOver(ref LocalID, t sem.TypeID, n syntax.NodeID, scoped bool) LocalID {
	owned := b.emit(Inst{Op: OpShare, Dst: b.temp(t), Args: []LocalID{ref}, Node: n})
	return b.emit(Inst{Op: OpNewCell, Dst: b.temp(t), Args: []LocalID{owned}, Node: n, Scoped: scoped})
}

// A coerced layer stops the peel: its steps belong to the value as a whole.
func peelNoOp(e *hir.Expr) *hir.Expr {
	for noOpLayer(e) {
		e = e.Args[0]
	}
	return e
}

func noOpLayer(e *hir.Expr) bool {
	if len(e.Coerce) > 0 {
		return false
	}
	return e.Kind == hir.Wrap || e.Kind == hir.DefinedWrap || e.Kind == hir.DefinedUnwrap
}

// Evaluates base and index once; a value with no place gets no write-back.
func (b *builder) exprPlace(base, inner *hir.Expr, sync bool) (place, bool) {
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
