package hir

import (
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// defaultArg lends a non-Copy argument to the thunk since the call itself still takes it afterwards.
func (l *lowerer) defaultArg(n syntax.NodeID) *Expr {
	thunk := l.info.Defaults[n]
	info := l.r.Fn(thunk)
	call, instantiated := l.info.Calls[n]
	generic := instantiated && call.Kind == sem.CallFn
	var ofCallee, ofThunk map[sem.EntityID]sem.TypeID
	out := l.node(n, Call)
	out.Ent = thunk
	if generic {
		ofCallee, ofThunk = l.r.InstanceSubst(info.DefaultOf, call.Inst), l.r.InstanceSubst(thunk, call.Inst)
		out.Passes, out.ConstArgs = call.Passes, call.ConstArgs
	}
	inst := func(t sem.TypeID, sub map[sem.EntityID]sem.TypeID) sem.TypeID {
		if sub == nil {
			return t
		}
		return l.r.Types.Subst(t, sub)
	}
	if info.Recv != sem.RecvNone {
		out.Kind = MethodCall
		out.Args = append(out.Args, l.argRef(n, -1, inst(l.r.Entity(info.SelfParam).Type, ofThunk)))
	}
	main := l.r.Fn(info.DefaultOf).Params
	for i, p := range info.Params {
		own := l.r.Entity(main[i]).Type
		ref := l.argRef(n, i, inst(own, ofCallee))
		if l.r.IsCopy(own) || l.r.Types.RefMut(own) {
			out.Args = append(out.Args, ref)
			continue
		}
		b := l.node(n, Borrow)
		b.Type, b.Args = inst(l.r.Entity(p).Type, ofThunk), []*Expr{ref}
		out.Args = append(out.Args, b)
	}
	if !info.DefaultTry {
		return out
	}
	out.Type = inst(l.r.Types.Node(info.Sig).Elem, ofThunk)
	try := l.node(n, Try)
	try.Args = []*Expr{out}
	switch {
	case call.Box != 0:
		try.BoxType = call.Box
	case call.Kind == sem.CallTry && len(call.Inst) == 1:
		try.BoxType = call.Inst[0]
	}
	return try
}

func (l *lowerer) argRef(n syntax.NodeID, index int, t sem.TypeID) *Expr {
	ref := l.node(n, ArgRef)
	ref.Type, ref.Index = t, index
	return ref
}
