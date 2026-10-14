package mir

import (
	"kigumi/internal/hir"
)

// Assigning self as a whole lands back in the caller's storage, but only
// when sync; async (sync false) cannot write back.
func (b *builder) recvCell(base *hir.Expr, sync bool) LocalID {
	if id, ok := b.refCellBase(base); ok {
		return id
	}
	n, inner := base.Node, peelNoOp(base)
	rt := b.r.Types.Ref(base.Type, true)
	p, hasPlace := b.exprPlace(base, inner, sync)
	var v LocalID
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
