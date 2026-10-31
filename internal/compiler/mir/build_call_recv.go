package mir

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// Assigning self as a whole lands back in the caller's storage, but only
// when sync; async (sync false) cannot write back.
func (b *builder) recvCell(base *hir.Expr, sync, inPlace bool) midir.LocalID {
	n, inner := base.Node, peelNoOp(base)
	rt := b.r.Types.Ref(base.Type, true)
	if id, ok := b.refCellBase(base); ok {
		if b.r.Types.Kind(base.Type) == sem.KRef {
			return id
		}
		return b.retagTo(id, b.fn.Locals[id].Type, rt, n)
	}
	p, hasPlace := b.exprPlace(base, inner, sync)
	if inPlace && sync && hasPlace {
		if cell, ok := b.lentRecvCell(p, base, inner, rt); ok {
			return cell
		}
	}
	var v midir.LocalID
	if hasPlace {
		v = b.retagTo(b.loadPlace(p), inner.Type, base.Type, n)
	} else {
		v = b.expr(base)
	}
	cell := b.lendCell(v, rt, n, sync)
	if sync {
		b.argFixups = append(b.argFixups, argFixup{recv: &p, elemType: base.Type, retag: inner.Type, cell: cell, node: n, depth: len(b.scopes)})
	}
	return cell
}

// A callee that leaves the receiver as it found it needs a view over the place, not a cell it hands back.
func (b *builder) lentRecvCell(p place, base, inner *hir.Expr, rt sem.TypeID) (midir.LocalID, bool) {
	var v midir.LocalID
	switch p.src.Kind {
	case hir.PlaceField, hir.PlaceIndex, hir.PlaceGlobal:
		v = b.loadPlace(p)
	default:
		if l := b.fn.Locals[p.id]; b.cellLocal(p.id) || l.Borrowed || b.r.Types.Kind(l.Type) == sem.KRef {
			return 0, false
		}
		v = p.id
	}
	n := base.Node
	ref := b.borrowRetag(v, inner.Type, rt, n)
	cell := b.emit(midir.Inst{Op: midir.OpNewCell, Dst: b.temp(rt), Args: []midir.LocalID{ref}, Node: n, Scoped: true, Lent: true})
	b.argFixups = append(b.argFixups, argFixup{cell: cell, node: n, depth: len(b.scopes), lent: true})
	return cell, true
}
