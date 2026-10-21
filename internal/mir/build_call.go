package mir

import (
	"kigumi/internal/hir"
	"kigumi/internal/sem"
)

func (b *builder) call(e *hir.Expr) LocalID {
	n, t := e.Node, e.Type
	// A nested call inside an argument flushes only its own write-backs,
	// leaving this call's pending until after it returns.
	mark := len(b.argFixups)
	if v, ok := b.fixedInline(e); ok {
		return v
	}
	switch e.Kind {
	case hir.Intrinsic:
		args := b.args(e, e.Args, e.EvalOrder, true, false, 0)
		return b.emit(Inst{Op: OpBuiltin, Dst: b.temp(t), Str: e.Name, Args: args, Node: n})
	case hir.Variant:
		args := b.args(e, e.Args, e.EvalOrder, true, false, 0)
		return b.emit(Inst{Op: OpVariant, Dst: b.temp(t), Ent: e.Ent, Type: t, Args: args, Node: n})
	case hir.MethodCall:
		if e.Cmp != "" {
			return b.witnessCompare(e, mark)
		}
		recv, rest := b.methodArgs(e)
		if v, ok := b.explicitDrop(e, recv, rest); ok {
			return v
		}
		args := append([]LocalID{recv}, rest...)
		if e.Async {
			args = append(args, b.witnessArgs(e.Passes, n)...)
			args = append(args, b.constArgs(e.ConstArgs, n)...)
			args = append(args, b.ctxArgs(e)...)
			fnv := b.emit(Inst{Op: OpFnItem, Dst: b.temp(b.r.Fn(e.Ent).Sig), Ent: e.Ent, Async: true, Node: n})
			return b.emit(Inst{Op: OpFutureNew, Dst: b.temp(t), Args: append([]LocalID{fnv}, args...), Node: n})
		}
		if e.Witness != 0 {
			args = append(args, b.ctxArgs(e)...)
			dst := b.emit(Inst{Op: OpCallValue, Dst: b.temp(t), Ent: e.Ent, Args: append([]LocalID{b.witnessLocal(e.Witness)}, args...), Node: n})
			b.applyArgFixups(mark)
			return dst
		}
		args = append(args, b.witnessArgs(e.Passes, n)...)
		args = append(args, b.constArgs(e.ConstArgs, n)...)
		args = append(args, b.ctxArgs(e)...)
		dst := b.emit(Inst{Op: OpCall, Dst: b.temp(t), Ent: e.Ent, Args: args, Node: n})
		b.applyArgFixups(mark)
		return dst
	case hir.StaticCall:
		args := b.args(e, e.Args, e.EvalOrder, true, true, e.Ent)
		args = append(args, b.ctxArgs(e)...)
		dst := b.emit(Inst{Op: OpCallValue, Dst: b.temp(t), Ent: e.Ent, Args: append([]LocalID{b.witnessLocal(e.Witness)}, args...), Node: n})
		b.applyArgFixups(mark)
		return dst
	case hir.Call:
		// An async call's future can outlive the call, so its args are moved, never reborrowed or fixed up.
		args := b.args(e, e.Args, e.EvalOrder, !e.Async, !e.Async, e.Ent)
		if e.Async {
			args = append(args, b.witnessArgs(e.Passes, n)...)
			args = append(args, b.constArgs(e.ConstArgs, n)...)
			args = append(args, b.ctxArgs(e)...)
			fnv := b.emit(Inst{Op: OpFnItem, Dst: b.temp(b.r.Fn(e.Ent).Sig), Ent: e.Ent, Async: true, Node: n})
			return b.emit(Inst{Op: OpFutureNew, Dst: b.temp(t), Args: append([]LocalID{fnv}, args...), Node: n})
		}
		args = append(args, b.witnessArgs(e.Passes, n)...)
		args = append(args, b.constArgs(e.ConstArgs, n)...)
		args = append(args, b.ctxArgs(e)...)
		dst := b.emit(Inst{Op: OpCall, Dst: b.temp(t), Ent: e.Ent, Args: args, Node: n})
		b.applyArgFixups(mark)
		return dst
	}
	fv, rest := b.valueCallArgs(e)
	args := append([]LocalID{fv}, rest...)
	if ct := b.fn.Locals[fv].Type; b.r.Types.IsCFn(ct) {
		dst := b.emit(Inst{Op: OpCallC, Dst: b.temp(t), Type: ct, Args: args, Node: n})
		b.applyArgFixups(mark)
		return dst
	}
	dst := b.emit(Inst{Op: OpCallValue, Dst: b.temp(t), Args: args, Node: n})
	b.applyArgFixups(mark)
	return dst
}

// ctxArgs evaluates a call's hidden provider arguments: plain reads, so
// unlike witnessArgs/constArgs they need no closure construction.
func (b *builder) ctxArgs(e *hir.Expr) []LocalID {
	var out []LocalID
	for _, a := range e.CtxArgs {
		out = append(out, b.expr(a))
	}
	return out
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
	full := b.evalArgs(1, len(e.Args), e.EvalOrder, func(i int) LocalID {
		if i == 0 {
			recv := b.receiver(e.Args[0], e.Ent, !e.Async, e.Witness == 0 && !b.r.ReplacesSelf(e.Ent))
			if later := laterArgs(e.Args, e.EvalOrder, 0); b.r.Fn(e.Ent).Recv == sem.RecvSelf && !mutBorrowsReceiver(e.Args[0], later) {
				recv = b.snapshot(recv, e.Args[0], e.Args[0].Node, later...)
			}
			return recv
		}
		return b.argValue(e.Args[i], i-1, e.Ent, true, !e.Async)
	})
	recv, rest := full[0], full[1:]
	if e.Variadic {
		rest = b.packVariadic(e, rest, b.r.Fn(e.Ent).Sig)
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
		rest = b.packVariadic(e, rest, b.valueSig(e.Args[0].Type))
	}
	return fv, rest
}

// A bare `&mut T` local reborrows instead of moving since the callee's borrow ends when the call returns.
// reborrow is false for async calls, whose future can outlive the call; sync false also skips placeArg's write-back.
func (b *builder) argValue(a *hir.Expr, idx int, ent sem.EntityID, reborrow, sync bool) LocalID {
	if v, ok := b.placeArg(a, sync); ok {
		return v
	}
	if v, ok := b.mutRefArg(a, idx, ent, sync); ok {
		return v
	}
	if inner := b.unparen(a); reborrow && inner.Kind == hir.Local && b.r.Types.Kind(inner.Type) == sem.KRef && !b.r.IsCopy(inner.Type) && b.declaredRef(ent, idx) {
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

// `mut self` takes a cell over the place, `move self` moves the value, `self` shares it; inPlace says the callee leaves the cell's content as it found it.
func (b *builder) receiver(base *hir.Expr, m sem.EntityID, sync, inPlace bool) LocalID {
	switch b.r.Fn(m).Recv {
	case sem.RecvMove:
		return b.exprMoved(base)
	case sem.RecvMut:
		return b.recvCell(base, sync, inPlace)
	}
	return b.expr(base)
}

func (b *builder) args(call *hir.Expr, exprs []*hir.Expr, order []int, reborrow, sync bool, ent sem.EntityID) []LocalID {
	out := b.evalArgs(0, len(exprs), order, func(i int) LocalID { return b.argValue(exprs[i], i, ent, reborrow, sync) })
	if call.Variadic {
		out = b.packVariadic(call, out, b.r.Fn(call.Ent).Sig)
	}
	return out
}
