package interp

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// evalArgs evaluates the plan's Eval temporaries in source order, then places them into parameter slots.
func (fr *frame) evalArgs(call sem.CallInfo) ([]Value, *ctrl) {
	plan := call.Plan
	vals := make([]Value, len(plan.Eval))
	for i, a := range plan.Eval {
		v, c := fr.expr(a)
		if c != nil {
			return nil, c
		}
		vals[i] = v
	}
	return fr.placeArgs(plan, vals, call), nil
}

// evalMethodArgs is evalArgs plus the receiver, run at its source position (after a piped left operand, else first).
func (fr *frame) evalMethodArgs(base syntax.NodeID, call sem.CallInfo) (Value, []Value, *ctrl) {
	plan := call.Plan
	vals := make([]Value, len(plan.Eval))
	start := 0
	if call.Piped != 0 {
		v, c := fr.expr(plan.Eval[0])
		if c != nil {
			return nil, nil, c
		}
		vals[0] = v
		start = 1
	}
	recv, c := fr.receiver(base, call.Callee)
	if c != nil {
		return nil, nil, c
	}
	for i := start; i < len(plan.Eval); i++ {
		v, c := fr.expr(plan.Eval[i])
		if c != nil {
			return nil, nil, c
		}
		vals[i] = v
	}
	return recv, fr.placeArgs(plan, vals, call), nil
}

// evalValueCallArgs is evalMethodArgs' counterpart for a call through a
// fn-typed value: the callee expression stands in for the receiver.
func (fr *frame) evalValueCallArgs(callee syntax.NodeID, call sem.CallInfo) (Value, []Value, *ctrl) {
	plan := call.Plan
	vals := make([]Value, len(plan.Eval))
	start := 0
	if call.Piped != 0 {
		v, c := fr.expr(plan.Eval[0])
		if c != nil {
			return nil, nil, c
		}
		vals[0] = v
		start = 1
	}
	fv, c := fr.expr(callee)
	if c != nil {
		return nil, nil, c
	}
	for i := start; i < len(plan.Eval); i++ {
		v, c := fr.expr(plan.Eval[i])
		if c != nil {
			return nil, nil, c
		}
		vals[i] = v
	}
	return fv, fr.placeArgs(plan, vals, call), nil
}

// placeArgs reads vals into parameter-slot order, applying each slot's borrow/move rule, then packs the variadic tail.
func (fr *frame) placeArgs(plan *sem.CallPlan, vals []Value, call sem.CallInfo) []Value {
	out := make([]Value, len(plan.Place))
	for i, idx := range plan.Place {
		a, v := plan.Eval[idx], vals[idx]
		if isBorrowExpr(fr.t, a) || fr.isReborrow(a, i, call) {
			out[i] = v
			continue
		}
		out[i] = fr.transfer(a, v)
	}
	if call.Variadic == sem.VariadicElements {
		fixed := len(fr.in.r.Types.Node(fr.in.r.Fn(call.Callee).Sig).Args) - 1
		if fixed < 0 {
			fixed = 0
		}
		if fixed > len(out) {
			fixed = len(out)
		}
		rest := &Array{Elems: append([]Value{}, out[fixed:]...)}
		out = append(out[:fixed], rest)
	}
	return out
}
