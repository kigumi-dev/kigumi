package sem

import "kigumi/internal/compiler/syntax"

// CallPlan is a call's two-layer argument schedule: Eval in source order, then Place into slots.
type CallPlan struct {
	// Eval lists explicit arguments in source order, piped left operand first, named args unwrapped.
	Eval []syntax.NodeID
	// Place[i] indexes Eval for fixed parameter i, then the variadic tail in the order written; -1 only inside an overload probe.
	Place []int
	// Order is Place's inverse: Order[k] is the parameter slot Eval[k] fills.
	Order []int
}

func newCallPlan(eval []syntax.NodeID, place []int) *CallPlan {
	order := make([]int, len(eval))
	for i, k := range place {
		if k >= 0 {
			order[k] = i
		}
	}
	return &CallPlan{Eval: eval, Place: place, Order: order}
}

func identityPlan(args []syntax.NodeID) *CallPlan {
	place := make([]int, len(args))
	for i := range place {
		place[i] = i
	}
	return newCallPlan(args, place)
}

// placed reads Eval through Place, rebuilding the call's arguments in
// parameter-slot order; a slot no argument fills is 0.
func (p *CallPlan) placed() []syntax.NodeID {
	out := make([]syntax.NodeID, len(p.Place))
	for i, k := range p.Place {
		if k >= 0 {
			out[i] = p.Eval[k]
		}
	}
	return out
}
