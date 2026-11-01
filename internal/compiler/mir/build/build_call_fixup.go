package build

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// argFixup is one cell the callee writes through and the storage its content
// goes back to: a scalar &mut argument's field or element, or recv's place.
type argFixup struct {
	recv     *place
	retag    sem.TypeID
	index    bool
	base     midir.LocalID
	idx      midir.LocalID
	field    int
	elemType sem.TypeID
	cell     midir.LocalID
	node     syntax.NodeID
	depth    int
	// lent: the cell only views the place, so nothing is taken out of it or stored back.
	lent bool
}

// placeArg boxes a scalar `&mut` argument for applyArgFixups to write back; skipped for async calls since their future can outlive this frame.
func (b *builder) placeArg(a *hir.Expr, sync bool) (midir.LocalID, bool) {
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
		cur := b.emit(midir.Inst{Op: midir.OpIndex, Dst: b.temp(inner.Type), Args: []midir.LocalID{base, idx}, Node: inner.Node})
		cell := b.boxScalarCell(b.retagTo(cur, inner.Type, scalarT, inner.Node), a.Type, a.Node)
		b.argFixups = append(b.argFixups, argFixup{index: true, base: base, idx: idx, elemType: scalarT, cell: cell, node: a.Node, depth: len(b.scopes)})
		return b.coerce(a.Coercion, a.Node, cell), true
	case hir.Field:
		base := b.storeBase(inner.Args[0])
		cur := b.emit(midir.Inst{Op: midir.OpField, Dst: b.temp(inner.Type), Args: []midir.LocalID{base}, Index: inner.Index, Node: inner.Node})
		cell := b.boxScalarCell(b.retagTo(cur, inner.Type, scalarT, inner.Node), a.Type, a.Node)
		b.argFixups = append(b.argFixups, argFixup{index: false, base: base, field: inner.Index, elemType: scalarT, cell: cell, node: a.Node, depth: len(b.scopes)})
		return b.coerce(a.Coercion, a.Node, cell), true
	}
	return 0, false
}

// retagTo aliases v from its own declared type to want when the two differ,
// the no-op boundary a defined-type wrap or `.value` crosses.
func (b *builder) retagTo(v midir.LocalID, from, want sem.TypeID, n syntax.NodeID) midir.LocalID {
	if from == want {
		return v
	}
	return b.emit(midir.Inst{Op: midir.OpAlias, Dst: b.temp(want), Args: []midir.LocalID{v}, Node: n})
}

// boxScalarCell mirrors build_expr.go's scalar-&mut boxing: OpShare takes an
// owned copy (scalars are Copy) and OpNewCell gives the callee somewhere to write.
func (b *builder) boxScalarCell(v midir.LocalID, t sem.TypeID, n syntax.NodeID) midir.LocalID {
	ref := b.emit(midir.Inst{Op: midir.OpBorrow, Dst: b.temp(t), Args: []midir.LocalID{v}, Index: 1, Node: n})
	owned := b.emit(midir.Inst{Op: midir.OpShare, Dst: b.temp(t), Args: []midir.LocalID{ref}, Node: n, Copy: true})
	return b.emit(midir.Inst{Op: midir.OpNewCell, Dst: b.temp(t), Args: []midir.LocalID{owned}, Node: n})
}

func (b *builder) abandonFixups(depth int) {
	for i := len(b.argFixups) - 1; i >= 0; i-- {
		fx := b.argFixups[i]
		if fx.depth <= depth {
			continue
		}
		if fx.lent {
			b.emit(midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{fx.cell}, Scoped: true, Node: fx.node})
			continue
		}
		v := b.emit(midir.Inst{Op: midir.OpCellTake, Dst: b.temp(fx.elemType), Args: []midir.LocalID{fx.cell}, Node: fx.node})
		b.emit(midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{fx.cell}, Scoped: true, Node: fx.node})
		b.emit(midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{v}, Scoped: true, Node: fx.node})
	}
}

// applyArgFixups must run right after the call that may have mutated each
// place returns; mark is where that call's own fixups start.
func (b *builder) applyArgFixups(mark int) {
	fixups := append([]argFixup(nil), b.argFixups[mark:]...)
	b.argFixups = b.argFixups[:mark]
	for _, fx := range fixups {
		drop := midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{fx.cell}, Scoped: true, Node: fx.node}
		if fx.lent {
			b.emit(drop)
			continue
		}
		if fx.recv != nil {
			v := b.emit(midir.Inst{Op: midir.OpCellTake, Dst: b.temp(fx.elemType), Args: []midir.LocalID{fx.cell}, Node: fx.node})
			b.emit(drop)
			if fx.recv.src == nil {
				b.emit(midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{v}, Scoped: true, Node: fx.node})
				continue
			}
			b.storePlace(*fx.recv, b.retagTo(v, fx.elemType, fx.retag, fx.node))
			continue
		}
		v := b.emit(midir.Inst{Op: midir.OpCellGet, Dst: b.temp(fx.elemType), Args: []midir.LocalID{fx.cell}, Node: fx.node})
		b.emit(drop)
		if fx.index {
			b.emit(midir.Inst{Op: midir.OpSetIndex, Args: []midir.LocalID{fx.base, fx.idx, v}, Node: fx.node})
			continue
		}
		b.emit(midir.Inst{Op: midir.OpSetField, Args: []midir.LocalID{fx.base, v}, Index: fx.field, Node: fx.node})
	}
}
