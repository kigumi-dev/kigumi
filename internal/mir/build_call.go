package mir

import (
	"kigumi/internal/hir"
	"kigumi/internal/sem"
)

func (b *builder) call(e *hir.Expr) LocalID {
	n, t := e.Node, e.Type
	switch e.Kind {
	case hir.Intrinsic:
		args := b.args(e, e.Args, e.EvalOrder, true, false, 0)
		return b.emit(Inst{Op: OpBuiltin, Dst: b.temp(t), Str: e.Name, Args: args, Node: n})
	case hir.Variant:
		args := b.args(e, e.Args, e.EvalOrder, true, false, 0)
		return b.emit(Inst{Op: OpVariant, Dst: b.temp(t), Ent: e.Ent, Type: t, Args: args, Node: n})
	case hir.MethodCall:
		recv, rest := b.methodArgs(e)
		args := append([]LocalID{recv}, rest...)
		if e.Async {
			fnv := b.emit(Inst{Op: OpFnItem, Dst: b.temp(b.r.Fn(e.Ent).Sig), Ent: e.Ent, Node: n})
			return b.emit(Inst{Op: OpBuiltin, Dst: b.temp(t), Str: "future", Args: append([]LocalID{fnv}, args...), Node: n})
		}
		if e.Witness != 0 {
			dst := b.emit(Inst{Op: OpCallValue, Dst: b.temp(t), Ent: e.Ent, Args: append([]LocalID{b.witnessLocal(e.Witness)}, args...), Node: n})
			b.applyArgFixups()
			return dst
		}
		args = append(args, b.witnessArgs(e.Passes, n)...)
		args = append(args, b.constArgs(e.ConstArgs, n)...)
		dst := b.emit(Inst{Op: OpCall, Dst: b.temp(t), Ent: e.Ent, Args: args, Node: n})
		b.applyArgFixups()
		return dst
	case hir.StaticCall:
		args := b.args(e, e.Args, e.EvalOrder, true, true, e.Ent)
		dst := b.emit(Inst{Op: OpCallValue, Dst: b.temp(t), Ent: e.Ent, Args: append([]LocalID{b.witnessLocal(e.Witness)}, args...), Node: n})
		b.applyArgFixups()
		return dst
	case hir.Call:
		// An async call's future can outlive the call, so its args are moved, never reborrowed or fixed up.
		args := b.args(e, e.Args, e.EvalOrder, !e.Async, !e.Async, e.Ent)
		if e.Async {
			fnv := b.emit(Inst{Op: OpFnItem, Dst: b.temp(b.r.Fn(e.Ent).Sig), Ent: e.Ent, Node: n})
			return b.emit(Inst{Op: OpBuiltin, Dst: b.temp(t), Str: "future", Args: append([]LocalID{fnv}, args...), Node: n})
		}
		args = append(args, b.witnessArgs(e.Passes, n)...)
		args = append(args, b.constArgs(e.ConstArgs, n)...)
		dst := b.emit(Inst{Op: OpCall, Dst: b.temp(t), Ent: e.Ent, Args: args, Node: n})
		b.applyArgFixups()
		return dst
	}
	fv, rest := b.valueCallArgs(e)
	args := append([]LocalID{fv}, rest...)
	if ct := b.fn.Locals[fv].Type; b.r.Types.IsCFn(ct) {
		dst := b.emit(Inst{Op: OpCallC, Dst: b.temp(t), Type: ct, Args: args, Node: n})
		b.applyArgFixups()
		return dst
	}
	dst := b.emit(Inst{Op: OpCallValue, Dst: b.temp(t), Args: args, Node: n})
	b.applyArgFixups()
	return dst
}

// evalOrder calls eval(i) for i in 0..n in the order sequence gives (nil: ascending), results indexed by i.
func evalOrder(n int, order []int, eval func(i int) LocalID) []LocalID {
	out := make([]LocalID, n)
	if order == nil {
		for i := 0; i < n; i++ {
			out[i] = eval(i)
		}
		return out
	}
	for _, i := range order {
		out[i] = eval(i)
	}
	return out
}

// methodArgs evaluates a MethodCall's receiver (Args[0]) and explicit arguments (Args[1:]) in e.EvalOrder's sequence.
func (b *builder) methodArgs(e *hir.Expr) (LocalID, []LocalID) {
	full := evalOrder(len(e.Args), e.EvalOrder, func(i int) LocalID {
		if i == 0 {
			return b.receiver(e.Args[0], e.Ent)
		}
		return b.argValue(e.Args[i], i-1, e.Ent, true, !e.Async)
	})
	recv, rest := full[0], full[1:]
	if e.Variadic {
		rest = b.packVariadic(e, rest)
	}
	return recv, rest
}

// valueCallArgs is methodArgs' counterpart for a call through a fn-typed value: the callee expression (Args[0]) stands in for the receiver.
func (b *builder) valueCallArgs(e *hir.Expr) (LocalID, []LocalID) {
	full := evalOrder(len(e.Args), e.EvalOrder, func(i int) LocalID {
		if i == 0 {
			return b.expr(e.Args[0])
		}
		return b.argValue(e.Args[i], i-1, 0, true, true)
	})
	fv, rest := full[0], full[1:]
	if e.Variadic {
		rest = b.packVariadic(e, rest)
	}
	return fv, rest
}

// A bare `&mut T` local reborrows instead of moving since the callee's borrow ends when the call returns.
// reborrow is false for async calls, whose future can outlive the call; sync false also skips placeArg's write-back.
func (b *builder) argValue(a *hir.Expr, idx int, ent sem.EntityID, reborrow, sync bool) LocalID {
	if v, ok := b.placeArg(a, sync); ok {
		return v
	}
	if reborrow && a.Kind == hir.Local && b.r.Types.Kind(a.Type) == sem.KRef && !b.r.IsCopy(a.Type) && b.declaredRef(ent, idx) {
		return b.expr(a)
	}
	return b.exprMoved(a)
}

// declaredRef is checked pre-instantiation: a type parameter unified to `&mut U` isn't written as `&mut T` in the signature, so E220 would miss the escaping reborrow otherwise.
// ent 0 (no fixed callee) falls back to an argument-type-only check.
func (b *builder) declaredRef(ent sem.EntityID, idx int) bool {
	if ent == 0 {
		return true
	}
	params := b.r.Fn(ent).Params
	if idx < 0 || idx >= len(params) {
		return false
	}
	return b.r.Types.Kind(b.r.Entity(params[idx]).Type) == sem.KRef
}

// receiver lowers a method receiver by its kind: `mut self` borrows the place, `move self` moves the value, `self` shares it.
func (b *builder) receiver(base *hir.Expr, m sem.EntityID) LocalID {
	info := b.r.Fn(m)
	if info.Recv == sem.RecvMove {
		return b.exprMoved(base)
	}
	v := b.expr(base)
	if info.Recv == sem.RecvMut {
		return b.emit(Inst{Op: OpBorrow, Dst: b.temp(b.r.Types.Ref(base.Type, true)), Args: []LocalID{v}, Index: 1, Node: base.Node})
	}
	return v
}

func (b *builder) args(call *hir.Expr, exprs []*hir.Expr, order []int, reborrow, sync bool, ent sem.EntityID) []LocalID {
	out := evalOrder(len(exprs), order, func(i int) LocalID { return b.argValue(exprs[i], i, ent, reborrow, sync) })
	if call.Variadic {
		out = b.packVariadic(call, out)
	}
	return out
}

// packVariadic groups a variadic call's trailing arguments into one array, per call.Ent's fixed arity.
func (b *builder) packVariadic(call *hir.Expr, out []LocalID) []LocalID {
	params := b.r.Types.Node(b.r.Fn(call.Ent).Sig).Args
	fixed := len(params) - 1
	if fixed < 0 {
		fixed = 0
	}
	if fixed > len(out) {
		fixed = len(out)
	}
	arr := b.emit(Inst{Op: OpArray, Dst: b.temp(params[len(params)-1]), Args: append([]LocalID{}, out[fixed:]...), Node: call.Node})
	return append(out[:fixed], arr)
}
