package mir

import (
	"math/big"

	"kigumi/internal/compiler/hir"
	"kigumi/internal/compiler/sem"
)

func (b *builder) fixedInline(e *hir.Expr) (LocalID, bool) {
	if e.Async || e.Witness != 0 || b.r.Entity(e.Ent).Kind != sem.EntFn || b.r.Entity(e.Ent).Flags&sem.EfStd == 0 {
		return 0, false
	}
	switch StdKey(b.r, e.Ent, true) {
	case "prelude.FixedArray.len":
		return b.fixedLen(e)
	case "prelude.FixedArray.get":
		return b.fixedGet(e)
	case "prelude.FixedArray.at":
		return b.fixedAt(e)
	case "prelude.FixedArray.set":
		return b.fixedSet(e)
	case "prelude.FixedArray.withMut":
		return b.fixedWithMut(e)
	case "prelude.FixedArray.filled":
		return b.fixedFill(e, false)
	case "prelude.FixedArray.zeroed":
		return b.fixedFill(e, true)
	}
	return 0, false
}

func (b *builder) usize(v int64, e *hir.Expr) LocalID {
	return b.emit(Inst{Op: OpConst, Dst: b.temp(sem.TyUsize), Lit: sem.Literal{Kind: sem.LitInt, Int: big.NewInt(v)}, Type: sem.TyUsize, Node: e.Node})
}

func (b *builder) fixedLen(e *hir.Expr) (LocalID, bool) {
	if len(e.Args[0].Coerce) > 0 || !b.r.FixedInline(e.Ent, e.Args[0].Type) {
		return 0, false
	}
	_, n, _ := b.r.FixedElem(e.Args[0].Type)
	b.expr(e.Args[0])
	return b.usize(n, e), true
}

// A zero fill names the element's zero only for a number, the one T a Zero witness is known for here.
func (b *builder) fixedFill(e *hir.Expr, zero bool) (LocalID, bool) {
	if !b.r.FixedInline(e.Ent, e.Type) {
		return 0, false
	}
	elem, n, _ := b.r.FixedElem(e.Type)
	var v LocalID
	switch {
	case !zero:
		v = b.args(e, e.Args, e.EvalOrder, true, true, e.Ent)[0]
	case b.r.Types.IsInteger(elem):
		v = b.emit(Inst{Op: OpConst, Dst: b.temp(elem), Lit: sem.Literal{Kind: sem.LitInt, Int: big.NewInt(0)}, Type: elem, Node: e.Node})
	case b.r.Types.IsFloat(elem):
		v = b.emit(Inst{Op: OpConst, Dst: b.temp(elem), Lit: sem.Literal{Kind: sem.LitFloat, Float: big.NewFloat(0)}, Type: elem, Node: e.Node})
	default:
		return 0, false
	}
	return b.emit(Inst{Op: OpBuiltin, Dst: b.temp(e.Type), Str: "fixed.fill", Args: []LocalID{v, b.usize(n, e)}, Node: e.Node}), true
}

// A `mut self` receiver must be a plain local, which the ops write in place.
func (b *builder) fixedArgs(e *hir.Expr, mut bool) ([]LocalID, bool) {
	recv := e.Args[0]
	if len(recv.Coerce) > 0 || !b.r.DirectFixed(recv.Type) {
		return nil, false
	}
	var fa LocalID
	if mut {
		if recv.Kind != hir.Local {
			return nil, false
		}
		fa = b.local(recv.Ent)
		if l := b.fn.Locals[fa]; l.Cell || l.Borrowed {
			return nil, false
		}
	}
	return evalOrder(len(e.Args), e.EvalOrder, func(i int) LocalID {
		if i == 0 {
			if mut {
				return fa
			}
			return b.expr(recv)
		}
		return b.argValue(e.Args[i], i-1, e.Ent, true, true)
	}), true
}
