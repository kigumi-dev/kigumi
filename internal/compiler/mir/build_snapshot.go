package mir

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// snapshot copies operand v, read by src, out when a later operand writes the local it reads, so the read keeps its source order.
// The copy is deep: a retain would still see a later in-place write such as a push.
func (b *builder) snapshot(v midir.LocalID, src *hir.Expr, n syntax.NodeID, later ...*hir.Expr) midir.LocalID {
	l := b.fn.Locals[v]
	if l.Ent == 0 {
		return b.snapshotPath(v, src, n, later)
	}
	if l.Cell || l.Borrowed {
		return v
	}
	t, cell := l.Type, false
	if b.r.Types.Kind(t) == sem.KRef {
		elem, mut := b.r.MutRefElem(t)
		if !mut || !b.cellLocal(v) {
			return v
		}
		t, cell = elem, true
	}
	if (b.r.HoldsResource(t) && !b.r.IsCopy(t)) || !b.writtenLater(localWrites{r: b.r, ent: l.Ent, moves: cell, lends: cell}, later) {
		return v
	}
	if cell {
		return b.emit(midir.Inst{Op: midir.OpCellGet, Dst: b.temp(t), Args: []midir.LocalID{v}, Node: n})
	}
	return b.emit(midir.Inst{Op: midir.OpShare, Dst: b.temp(t), Args: []midir.LocalID{v}, Copy: true, Node: n})
}

// snapshotRecv is snapshot for a `self` receiver, which also deep-copies a `&mut` referent other than a resource that a later
// operand writes through, even one lent as a cell, since the cell shares a record a field store or push writes in place.
func (b *builder) snapshotRecv(v midir.LocalID, src *hir.Expr, n syntax.NodeID, later []*hir.Expr) midir.LocalID {
	l := b.fn.Locals[v]
	elem, mut := b.r.MutRefElem(l.Type)
	if l.Ent == 0 || l.Cell || l.Borrowed || !mut || b.r.HoldsResource(elem) && !b.r.IsCopy(elem) {
		return b.snapshot(v, src, n, later...)
	}
	if passedAgain(l.Ent, later) || !b.writtenLater(localWrites{r: b.r, ent: l.Ent, moves: true, lends: true}, later) {
		return v
	}
	if b.cellLocal(v) {
		v = b.emit(midir.Inst{Op: midir.OpCellGet, Dst: b.temp(elem), Args: []midir.LocalID{v}, Node: n})
		if b.r.IsCopy(elem) && !b.copyRecord(elem) {
			return v
		}
	} else {
		v = b.retagTo(v, l.Type, elem, n)
	}
	return b.emit(midir.Inst{Op: midir.OpShare, Dst: b.temp(elem), Args: []midir.LocalID{v}, Copy: true, Node: n})
}

// passedAgain: the reference handed on as an argument of the same call is the borrow checker's exclusivity conflict.
func passedAgain(ent sem.EntityID, later []*hir.Expr) bool {
	for _, x := range later {
		if x != nil && x.Kind == hir.Local && x.Ent == ent {
			return true
		}
	}
	return false
}

// snapshotPath copies a field or element read, or a cell read, whose root local a later operand writes; a Copy read is already a value,
// except a Copy record's, which shares the record a later field store writes in place, also from a closure a later call runs.
func (b *builder) snapshotPath(v midir.LocalID, src *hir.Expr, n syntax.NodeID, later []*hir.Expr) midir.LocalID {
	t := b.fn.Locals[v].Type
	root := pathRoot(src)
	if root == 0 && src != nil && src.Kind == hir.Local && b.copyRecord(t) {
		root = src.Ent
	}
	if root == 0 || b.r.IsCopy(t) && !b.copyRecord(t) || b.r.HoldsResource(t) && !b.r.IsCopy(t) {
		return v
	}
	_, lends := b.r.MutRefElem(b.r.Entity(root).Type)
	id, ok := b.locals[root]
	if !b.writtenLater(localWrites{r: b.r, ent: root, moves: lends, lends: lends, calls: ok && b.fn.Locals[id].Cell}, later) {
		return v
	}
	return b.emit(midir.Inst{Op: midir.OpShare, Dst: b.temp(t), Args: []midir.LocalID{v}, Copy: true, Node: n})
}

func (b *builder) copyRecord(t sem.TypeID) bool {
	if b.r.Types.Kind(t) != sem.KNamed {
		return false
	}
	d := b.r.TypeDecl(b.r.Types.Node(t).Ent)
	return d != nil && d.Form == sem.FormRecord
}

// pathRoot names the local a field or element read starts from, or 0.
func pathRoot(e *hir.Expr) sem.EntityID {
	if e == nil || (e.Kind != hir.Field && e.Kind != hir.Index) {
		return 0
	}
	for {
		switch e.Kind {
		case hir.Local:
			return e.Ent
		case hir.Field, hir.Index, hir.Wrap, hir.DefinedWrap, hir.DefinedUnwrap:
			e = e.Args[0]
		default:
			return 0
		}
	}
}

func (b *builder) writtenLater(w localWrites, later []*hir.Expr) bool {
	for _, x := range later {
		if w.expr(x) {
			return true
		}
	}
	return false
}

// laterArgs lists the arguments evaluated after argument i under order (nil: ascending).
func laterArgs(args []*hir.Expr, order []int, i int) []*hir.Expr {
	if order == nil {
		return args[i+1:]
	}
	for k, j := range order {
		if j == i {
			out := make([]*hir.Expr, 0, len(order)-k-1)
			for _, n := range order[k+1:] {
				out = append(out, args[n])
			}
			return out
		}
	}
	return nil
}
