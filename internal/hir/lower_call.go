package hir

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

func (l *lowerer) binary(n syntax.NodeID) *Expr {
	bin := l.t.View(n).AsBinary()
	lhs, rhs := bin.Left().ID(), bin.Right().ID()
	if lit, ok := l.r.LiteralOf(l.t, n); ok && (lit.Kind == sem.LitInt || lit.Kind == sem.LitFloat) {
		return l.literal(n)
	}
	call := l.info.Calls[n]
	op := bin.Op().Text()
	switch {
	case call.Kind == sem.CallFallback:
		out := l.node(n, Fallback)
		out.Ent = call.Callee
		out.HitLift = call.HitLift
		out.Args = []*Expr{l.expr(lhs)}
		// The miss side of `a || (x) => b` binds the miss value.
		if l.t.Kind(rhs) == syntax.Lambda {
			lam := l.t.View(rhs).AsLambda()
			out.Param = l.decl(l.info.Defs[lam.Params().ItemID(0)])
			out.Args = append(out.Args, l.expr(lam.Body().ID()))
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
	return l.callWith(n, l.t.View(n).AsCallLike().Callee().ID(), call)
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
		base := l.t.View(memberNode(l.t, callee)).AsMember().Base().ID()
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
		return t.View(callee).AsBracket().Base().ID()
	}
	return callee
}

func (l *lowerer) control(n syntax.NodeID) *Expr {
	v := l.t.View(n)
	switch v.Kind() {
	case syntax.IfLetExpr:
		s := v.AsIfLet()
		out := l.node(n, IfLet)
		out.Cond = l.expr(s.Init().ID())
		out.Pat = l.pattern(s.Pattern().ID())
		if g := s.Guard(); g.Valid() {
			out.Guard = l.expr(g.ID())
		}
		out.Then = l.block(s.Then().ID())
		if els := s.Else(); els.Valid() {
			out.Else = l.expr(els.ID())
		}
		return out
	case syntax.MatchExpr:
		m := v.AsMatch()
		out := l.node(n, Match)
		out.Args = []*Expr{l.expr(m.Scrutinee().ID())}
		for av := range m.Arms().Views() {
			arm := av.AsArm()
			a := &Arm{Pat: l.pattern(arm.Pattern().ID())}
			if g := arm.Guard(); g.Valid() {
				a.Guard = l.expr(g.ID())
			}
			a.Body = l.expr(arm.Body().ID())
			out.Arms = append(out.Arms, a)
		}
		return out
	}
	f := v.AsFor()
	out := l.node(n, Loop)
	if p := f.Pattern(); p.Valid() {
		out.Pat = l.pattern(p.ID())
	}
	if head := f.Head(); head.Valid() {
		out.Cond = l.expr(head.ID())
	}
	out.Block = l.block(f.Body().ID())
	return out
}
