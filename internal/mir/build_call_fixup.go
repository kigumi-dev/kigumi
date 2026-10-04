package mir

import (
	"kigumi/internal/hir"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// argFixup is a scalar &mut argument's place, boxed into a cell since the array/record itself holds a plain scalar.
type argFixup struct {
	index    bool
	base     LocalID
	idx      LocalID
	field    int
	elemType sem.TypeID
	cell     LocalID
	node     syntax.NodeID
}

// placeArg boxes a scalar `&mut` argument for applyArgFixups to write back; skipped for async calls since their future can outlive this frame.
func (b *builder) placeArg(a *hir.Expr, sync bool) (LocalID, bool) {
	if !sync || a.Kind != hir.Borrow || !a.Mut {
		return 0, false
	}
	scalarT, ok := b.r.ScalarRefElem(a.Type)
	if !ok {
		return 0, false
	}
	// `&mut handle.value` boxes the field underneath handle like an ordinary
	// scalar field; retagTo bridges the two declared types.
	inner := a.Args[0]
	for inner.Kind == hir.DefinedWrap || inner.Kind == hir.DefinedUnwrap {
		inner = inner.Args[0]
	}
	switch inner.Kind {
	case hir.Index:
		base := b.expr(inner.Args[0])
		idx := b.expr(inner.Args[1])
		cur := b.emit(Inst{Op: OpIndex, Dst: b.temp(inner.Type), Args: []LocalID{base, idx}, Node: inner.Node})
		cell := b.boxScalarCell(b.retagTo(cur, inner.Type, scalarT, inner.Node), a.Type, a.Node)
		b.argFixups = append(b.argFixups, argFixup{index: true, base: base, idx: idx, elemType: scalarT, cell: cell, node: a.Node})
		return b.coerce(a.Coercion, a.Node, cell), true
	case hir.Field:
		base := b.expr(inner.Args[0])
		cur := b.emit(Inst{Op: OpField, Dst: b.temp(inner.Type), Args: []LocalID{base}, Index: inner.Index, Node: inner.Node})
		cell := b.boxScalarCell(b.retagTo(cur, inner.Type, scalarT, inner.Node), a.Type, a.Node)
		b.argFixups = append(b.argFixups, argFixup{index: false, base: base, field: inner.Index, elemType: scalarT, cell: cell, node: a.Node})
		return b.coerce(a.Coercion, a.Node, cell), true
	}
	return 0, false
}

// retagTo aliases v from its own declared type to want when the two differ,
// the no-op boundary a defined-type wrap or `.value` crosses.
func (b *builder) retagTo(v LocalID, from, want sem.TypeID, n syntax.NodeID) LocalID {
	if from == want {
		return v
	}
	return b.emit(Inst{Op: OpAlias, Dst: b.temp(want), Args: []LocalID{v}, Node: n})
}

// boxScalarCell mirrors build_expr.go's scalar-&mut boxing: OpShare takes an
// owned copy (scalars are Copy) and OpNewCell gives the callee somewhere to write.
func (b *builder) boxScalarCell(v LocalID, t sem.TypeID, n syntax.NodeID) LocalID {
	ref := b.emit(Inst{Op: OpBorrow, Dst: b.temp(t), Args: []LocalID{v}, Index: 1, Node: n})
	owned := b.emit(Inst{Op: OpShare, Dst: b.temp(t), Args: []LocalID{ref}, Node: n, Copy: true})
	return b.emit(Inst{Op: OpNewCell, Dst: b.temp(t), Args: []LocalID{owned}, Node: n})
}

// applyArgFixups must run right after the call that may have mutated each place returns.
func (b *builder) applyArgFixups() {
	fixups := b.argFixups
	b.argFixups = nil
	for _, fx := range fixups {
		v := b.emit(Inst{Op: OpCellGet, Dst: b.temp(fx.elemType), Args: []LocalID{fx.cell}, Node: fx.node})
		if fx.index {
			b.emit(Inst{Op: OpSetIndex, Args: []LocalID{fx.base, fx.idx, v}, Node: fx.node})
		} else {
			b.emit(Inst{Op: OpSetField, Args: []LocalID{fx.base, v}, Index: fx.field, Node: fx.node})
		}
	}
}
