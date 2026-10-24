package mir

import (
	"kigumi/internal/hir"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// snapshot copies operand v, read by src, out when a later operand writes the local it reads, so the read keeps its source order.
// The copy is deep: a retain would still see a later in-place write such as a push.
func (b *builder) snapshot(v LocalID, src *hir.Expr, n syntax.NodeID, later ...*hir.Expr) LocalID {
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
	if (b.r.HoldsResource(t) && !b.r.IsCopy(t)) || !b.writtenLater(l.Ent, later) {
		return v
	}
	if cell {
		return b.emit(Inst{Op: OpCellGet, Dst: b.temp(t), Args: []LocalID{v}, Node: n})
	}
	return b.emit(Inst{Op: OpShare, Dst: b.temp(t), Args: []LocalID{v}, Copy: true, Node: n})
}

// snapshotPath copies a field or element read whose root local a later operand writes; a Copy read is already a value,
// except a Copy record's, which shares the record a later field store writes in place.
func (b *builder) snapshotPath(v LocalID, src *hir.Expr, n syntax.NodeID, later []*hir.Expr) LocalID {
	t := b.fn.Locals[v].Type
	root := pathRoot(src)
	if root == 0 || b.r.IsCopy(t) && !b.copyRecord(t) || b.r.HoldsResource(t) && !b.r.IsCopy(t) || !b.writtenLater(root, later) {
		return v
	}
	return b.emit(Inst{Op: OpShare, Dst: b.temp(t), Args: []LocalID{v}, Copy: true, Node: n})
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

func (b *builder) writtenLater(ent sem.EntityID, later []*hir.Expr) bool {
	for _, x := range later {
		if writesLocal(b.r, x, ent) {
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
