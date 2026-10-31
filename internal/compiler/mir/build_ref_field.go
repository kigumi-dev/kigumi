package mir

import (
	"kigumi/internal/compiler/hir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// The alias keeps ownership from counting the lent field as moved out of its holder.
func (b *builder) fieldReborrow(a *hir.Expr, idx int, ent sem.EntityID, reborrow bool) (LocalID, bool) {
	inner := b.unparen(a)
	if !reborrow || inner.Kind != hir.Field || !b.r.Types.RefMut(inner.Type) || !b.declaredRef(ent, idx) {
		return 0, false
	}
	return b.emit(Inst{Op: OpAlias, Dst: b.temp(a.Type), Args: []LocalID{b.expr(a)}, Node: a.Node}), true
}

// A holder keeps no count of a `&mut` field, so the read takes its own and the slot needs no clearing.
func (b *builder) refFieldMove(base LocalID, t sem.TypeID, idx int, n syntax.NodeID) LocalID {
	v := b.emit(Inst{Op: OpField, Dst: b.temp(t), Args: []LocalID{base}, Index: idx, Node: n})
	return b.emit(Inst{Op: OpMove, Dst: b.temp(t), Args: []LocalID{v}, Node: n})
}
