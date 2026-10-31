package mir

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// baseLocal peels defined-type wrap/unwrap layers (same storage as their
// receiver) down to the hir.Local underneath, if any.
func baseLocal(e *hir.Expr) (*hir.Expr, bool) {
	for e.Kind == hir.DefinedWrap || e.Kind == hir.DefinedUnwrap {
		e = e.Args[0]
	}
	if e.Kind != hir.Local {
		return nil, false
	}
	return e, true
}

func (b *builder) borrowExpr(e *hir.Expr, n syntax.NodeID, t sem.TypeID) midir.LocalID {
	// A scalar `&mut` of a bare local/parameter aliases that local's own
	// cell (sem flags it EfAddrTaken) instead of boxing a disconnected copy.
	if e.Mut {
		if _, ok := b.r.ScalarRefElem(t); ok {
			if base, ok := baseLocal(e.Args[0]); ok {
				if b.r.Types.RefMut(base.Type) {
					return b.retagTo(b.expr(base), base.Type, t, n)
				}
				if id := b.local(base.Ent); b.fn.Locals[id].Cell {
					ref := b.emit(midir.Inst{Op: midir.OpBorrow, Dst: b.temp(b.r.Types.Ref(base.Type, true)), Args: []midir.LocalID{id}, Index: 1, Node: n})
					if base != e.Args[0] {
						ref = b.emit(midir.Inst{Op: midir.OpAlias, Dst: b.temp(t), Args: []midir.LocalID{ref}, Node: n})
					}
					return ref
				}
			}
		}
	}
	mut := 0
	if e.Mut {
		mut = 1
	}
	var ref midir.LocalID
	if base := definedBase(e.Args[0]); e.Mut && base != e.Args[0] && b.r.Types.Kind(base.Type) != sem.KRef && !b.scalarRef(t) {
		ref = b.borrowRetag(b.expr(base), base.Type, t, n)
	} else if k := e.Args[0].Kind; e.Mut && (k == hir.Field || k == hir.Index) {
		ref = b.emit(midir.Inst{Op: midir.OpBorrow, Dst: b.temp(t), Args: []midir.LocalID{b.storeBase(e.Args[0])}, Index: mut, Node: n})
	} else {
		ref = b.emit(midir.Inst{Op: midir.OpBorrow, Dst: b.temp(t), Args: []midir.LocalID{b.expr(e.Args[0])}, Index: mut, Node: n})
	}
	// A scalar `&mut` must be stored through by replacing its referent, so
	// it is boxed; the box takes its own reference since a borrow owns nothing.
	if e.Mut {
		if _, ok := b.r.ScalarRefElem(t); ok {
			owned := b.emit(midir.Inst{Op: midir.OpShare, Dst: b.temp(t), Args: []midir.LocalID{ref}, Node: n, Copy: true})
			return b.emit(midir.Inst{Op: midir.OpNewCell, Dst: b.temp(t), Args: []midir.LocalID{owned}, Node: n})
		}
	}
	return ref
}

func (b *builder) scalarRef(t sem.TypeID) bool {
	_, ok := b.r.ScalarRefElem(t)
	return ok
}

// borrowRetag lends v in place under its own declared type and retags the reference, since retagging v first would copy a direct value.
func (b *builder) borrowRetag(v midir.LocalID, from, want sem.TypeID, n syntax.NodeID) midir.LocalID {
	ref := b.emit(midir.Inst{Op: midir.OpBorrow, Dst: b.temp(b.r.Types.Ref(from, true)), Args: []midir.LocalID{v}, Index: 1, Node: n})
	return b.retagTo(ref, b.r.Types.Ref(from, true), want, n)
}

// derefsRef reports a retag that reads a Copy value through a reference, which must not leave as an alias of what it borrows.
func (b *builder) derefsRef(from, to sem.TypeID) bool {
	return b.r.Types.Kind(from) == sem.KRef && b.r.Types.Kind(to) != sem.KRef && b.r.IsCopy(to)
}
