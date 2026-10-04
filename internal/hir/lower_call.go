package hir

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

func (l *lowerer) binary(n syntax.NodeID) *Expr {
	node := l.t.Nodes[n]
	lhs, rhs := syntax.NodeID(node.Lhs), syntax.NodeID(node.Rhs)
	if lit, ok := l.r.LiteralOf(l.t, n); ok && (lit.Kind == sem.LitInt || lit.Kind == sem.LitFloat) {
		return l.literal(n)
	}
	call := l.info.Calls[n]
	op := l.t.TokText(node.Tok)
	switch {
	case call.Kind == sem.CallFallback:
		out := l.node(n, Fallback)
		out.Ent = call.Callee
		out.Args = []*Expr{l.expr(lhs)}
		// The miss side of `a || (x) => b` binds the miss value.
		if l.t.Kind(rhs) == syntax.Lambda {
			ln := l.t.Nodes[rhs]
			out.Param = l.decl(l.info.Defs[l.t.Children(syntax.NodeID(ln.Lhs))[0]])
			out.Args = append(out.Args, l.expr(syntax.NodeID(ln.Rhs)))
		} else {
			out.Args = append(out.Args, l.expr(rhs))
		}
		return out
	case call.Kind == sem.CallPipe:
		// A call on the right already takes the left operand as its first
		// argument; only a lambda is applied here.
		if l.t.Kind(rhs) != syntax.Lambda {
			out := l.node(n, Wrap)
			out.Args = []*Expr{l.expr(rhs)}
			return out
		}
		out := l.node(n, Pipe)
		out.Args = []*Expr{l.expr(lhs), l.expr(rhs)}
		return out
	case call.Piped != 0:
		// `a |> f`: the name on the right called with the left operand.
		return l.callWith(n, rhs, call)
	case op == "&&":
		out := l.node(n, And)
		out.Args = []*Expr{l.expr(lhs), l.expr(rhs)}
		return out
	}
	if call.Kind == sem.CallMethod {
		return l.protoBinary(n, op, call, lhs, rhs)
	}
	out := l.node(n, Binary)
	out.Name, out.Args = op, []*Expr{l.expr(lhs), l.expr(rhs)}
	if call.Kind == sem.CallOperator {
		out.Ent = call.Callee
	}
	return out
}

func (l *lowerer) call(n syntax.NodeID) *Expr {
	call := l.info.Calls[n]
	return l.callWith(n, syntax.NodeID(l.t.Nodes[n].Lhs), call)
}

// callWith builds Args in parameter-slot order from the sem Plan and sets EvalOrder to replay it in source order.
func (l *lowerer) callWith(n, callee syntax.NodeID, call sem.CallInfo) *Expr {
	plan := call.Plan
	args := make([]*Expr, len(plan.Place))
	for i, k := range plan.Place {
		args[i] = l.expr(plan.Eval[k])
	}
	out := l.node(n, Call)
	out.Ent, out.Args, out.Variadic = call.Callee, args, call.Variadic == sem.VariadicElements
	switch call.Kind {
	case sem.CallIntrinsic:
		out.Kind, out.Name = Intrinsic, l.r.Entity(call.Callee).Name
		out.EvalOrder = plan.Order
	case sem.CallVariant:
		out.Kind = Variant
		out.EvalOrder = plan.Order
	case sem.CallDefinedWrap:
		out.Kind = DefinedWrap
		out.EvalOrder = plan.Order
	case sem.CallMethod:
		base := syntax.NodeID(l.t.Nodes[memberNode(l.t, callee)].Lhs)
		out.Kind, out.Witness, out.Passes, out.ConstArgs = MethodCall, call.Witness, call.Passes, call.ConstArgs
		out.Args = append([]*Expr{l.expr(base)}, args...)
		out.EvalOrder = prefixEvalOrder(plan, call.Piped != 0)
		out.Async = l.r.Fn(call.Callee).Declared&sem.EffAsync != 0
	case sem.CallStatic:
		out.Kind, out.Witness = StaticCall, call.Witness
		out.EvalOrder = plan.Order
	case sem.CallFn, sem.CallAssoc, sem.CallOperator:
		out.Passes, out.ConstArgs = call.Passes, call.ConstArgs
		out.EvalOrder = plan.Order
		out.Async = l.r.Fn(call.Callee).Declared&sem.EffAsync != 0
	default:
		out.Kind = ValueCall
		out.Args = append([]*Expr{l.expr(callee)}, args...)
		out.EvalOrder = prefixEvalOrder(plan, call.Piped != 0)
	}
	for _, b := range l.info.CtxCall[n] {
		out.CtxArgs = append(out.CtxArgs, l.ctxBindExpr(n, b))
	}
	return out
}

// prefixEvalOrder shifts plan.Order for an Args array with a receiver/callee prepended at index 0.
func prefixEvalOrder(plan *sem.CallPlan, piped bool) []int {
	n := len(plan.Order)
	out := make([]int, 0, n+1)
	start := 0
	if piped {
		out = append(out, plan.Order[0]+1, 0)
		start = 1
	} else {
		out = append(out, 0)
	}
	for k := start; k < n; k++ {
		out = append(out, plan.Order[k]+1)
	}
	return out
}

// memberNode is the member expression of a method callee, stepping past
// explicit type arguments (`recv.get[T](...)`).
func memberNode(t *syntax.Tree, callee syntax.NodeID) syntax.NodeID {
	if t.Kind(callee) == syntax.BracketExpr {
		return syntax.NodeID(t.Nodes[callee].Lhs)
	}
	return callee
}

func (l *lowerer) control(n syntax.NodeID) *Expr {
	node := l.t.Nodes[n]
	slot := func(name string) syntax.NodeID { return syntax.NodeID(l.t.Slot(n, name)) }
	switch node.Kind {
	case syntax.IfLetExpr:
		out := l.node(n, IfLet)
		out.Cond = l.expr(slot("init"))
		out.Pat = l.pattern(slot("pattern"))
		if g := slot("guard"); g != 0 {
			out.Guard = l.expr(g)
		}
		out.Then = l.block(slot("then"))
		if els := slot("else"); els != 0 {
			out.Else = l.expr(els)
		}
		return out
	case syntax.MatchExpr:
		out := l.node(n, Match)
		out.Args = []*Expr{l.expr(syntax.NodeID(node.Lhs))}
		for _, arm := range l.t.Children(syntax.NodeID(node.Rhs)) {
			a := &Arm{Pat: l.pattern(syntax.NodeID(l.t.Slot(arm, "pattern")))}
			if g := syntax.NodeID(l.t.Slot(arm, "guard")); g != 0 {
				a.Guard = l.expr(g)
			}
			a.Body = l.expr(syntax.NodeID(l.t.Slot(arm, "body")))
			out.Arms = append(out.Arms, a)
		}
		return out
	}
	out := l.node(n, Loop)
	if p := slot("pattern"); p != 0 {
		out.Pat = l.pattern(p)
	}
	if head := slot("head"); head != 0 {
		out.Cond = l.expr(head)
	}
	out.Block = l.block(slot("body"))
	return out
}
