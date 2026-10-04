package sem

import "kigumi/internal/syntax"

// CallPlan is a call's two-layer argument schedule: Eval in source order, then Place into slots.
type CallPlan struct {
	// Eval lists explicit arguments in source order, piped left operand first, named args unwrapped.
	Eval []syntax.NodeID
	// Place[i] indexes Eval for fixed parameter i, then the variadic tail in the order written.
	Place []int
	// Order is Place's inverse: Order[k] is the parameter slot Eval[k] fills.
	Order []int
}

func newCallPlan(eval []syntax.NodeID, place []int) *CallPlan {
	order := make([]int, len(place))
	for i, k := range place {
		order[k] = i
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
// parameter-slot order.
func (p *CallPlan) placed() []syntax.NodeID {
	out := make([]syntax.NodeID, len(p.Place))
	for i, k := range p.Place {
		out[i] = p.Eval[k]
	}
	return out
}
