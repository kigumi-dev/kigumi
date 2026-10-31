package mir

import (
	"kigumi/internal/compiler/hir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// argFixup is one cell the callee writes through and the storage its content
// goes back to: a scalar &mut argument's field or element, or recv's place.
type argFixup struct {
	recv     *place
	retag    sem.TypeID
	index    bool
	base     LocalID
	idx      LocalID
	field    int
	elemType sem.TypeID
	cell     LocalID
	node     syntax.NodeID
	depth    int
	// lent: the cell only views the place, so nothing is taken out of it or stored back.
	lent bool
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
		base := b.storeBase(inner.Args[0])
		idx := b.expr(inner.Args[1])
		cur := b.emit(Inst{Op: OpIndex, Dst: b.temp(inner.Type), Args: []LocalID{base, idx}, Node: inner.Node})
		cell := b.boxScalarCell(b.retagTo(cur, inner.Type, scalarT, inner.Node), a.Type, a.Node)
		b.argFixups = append(b.argFixups, argFixup{index: true, base: base, idx: idx, elemType: scalarT, cell: cell, node: a.Node, depth: len(b.scopes)})
		return b.coerce(a.Coercion, a.Node, cell), true
	case hir.Field:
		base := b.storeBase(inner.Args[0])
		cur := b.emit(Inst{Op: OpField, Dst: b.temp(inner.Type), Args: []LocalID{base}, Index: inner.Index, Node: inner.Node})
		cell := b.boxScalarCell(b.retagTo(cur, inner.Type, scalarT, inner.Node), a.Type, a.Node)
		b.argFixups = append(b.argFixups, argFixup{index: false, base: base, field: inner.Index, elemType: scalarT, cell: cell, node: a.Node, depth: len(b.scopes)})
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

func (b *builder) abandonFixups(depth int) {
	for i := len(b.argFixups) - 1; i >= 0; i-- {
		fx := b.argFixups[i]
		if fx.depth <= depth {
			continue
		}
		if fx.lent {
			b.emit(Inst{Op: OpDrop, Args: []LocalID{fx.cell}, Scoped: true, Node: fx.node})
			continue
		}
		v := b.emit(Inst{Op: OpCellTake, Dst: b.temp(fx.elemType), Args: []LocalID{fx.cell}, Node: fx.node})
		b.emit(Inst{Op: OpDrop, Args: []LocalID{fx.cell}, Scoped: true, Node: fx.node})
		b.emit(Inst{Op: OpDrop, Args: []LocalID{v}, Scoped: true, Node: fx.node})
	}
}

// applyArgFixups must run right after the call that may have mutated each
// place returns; mark is where that call's own fixups start.
func (b *builder) applyArgFixups(mark int) {
	fixups := append([]argFixup(nil), b.argFixups[mark:]...)
	b.argFixups = b.argFixups[:mark]
	for _, fx := range fixups {
		drop := Inst{Op: OpDrop, Args: []LocalID{fx.cell}, Scoped: true, Node: fx.node}
		if fx.lent {
			b.emit(drop)
			continue
		}
		if fx.recv != nil {
			v := b.emit(Inst{Op: OpCellTake, Dst: b.temp(fx.elemType), Args: []LocalID{fx.cell}, Node: fx.node})
			b.emit(drop)
			if fx.recv.src == nil {
				b.emit(Inst{Op: OpDrop, Args: []LocalID{v}, Scoped: true, Node: fx.node})
				continue
			}
			b.storePlace(*fx.recv, b.retagTo(v, fx.elemType, fx.retag, fx.node))
			continue
		}
		v := b.emit(Inst{Op: OpCellGet, Dst: b.temp(fx.elemType), Args: []LocalID{fx.cell}, Node: fx.node})
		b.emit(drop)
		if fx.index {
			b.emit(Inst{Op: OpSetIndex, Args: []LocalID{fx.base, fx.idx, v}, Node: fx.node})
			continue
		}
		b.emit(Inst{Op: OpSetField, Args: []LocalID{fx.base, v}, Index: fx.field, Node: fx.node})
	}
}
