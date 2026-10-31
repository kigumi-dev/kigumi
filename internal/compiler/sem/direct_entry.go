package sem

import "kigumi/internal/compiler/syntax"

// A boxed caller reaches a direct-signature function through its entry, which boxes the result.
const (
	whyEntryBox      = 14
	whyDynamicResult = 15
	whyWitnessCross  = 16
	whyWitnessResult = 17
)

func boxWhy(why uint8) bool {
	return why == whyEntryBox || why == whyDynamicResult || why == whyWitnessCross || why == whyWitnessResult
}

// A specialized instance calls the witness directly, so only the built program knows whether it boxes.
func witnessWhy(why uint8) bool { return why == whyWitnessCross || why == whyWitnessResult }

// BoxAllocates reports whether boxing a t allocates: a payload-free variant's box is an immortal.
func (r *Result) BoxAllocates(t TypeID) bool {
	if !r.DirectAgg(t) {
		return false
	}
	if !r.DirectAdt(t) {
		return true
	}
	for _, v := range r.AggVariants(t) {
		if len(r.AggPayload(t, v)) > 0 {
			return true
		}
	}
	return false
}

// EntryBoxes is the type fn's entry boxes when it returns ret, 0 when that allocates nothing.
func (r *Result) EntryBoxes(fn EntityID, ret TypeID) TypeID {
	if r.ScalarBoxes(ret) {
		return ret
	}
	if t := r.DirectResultType(fn, ret); t != 0 && r.BoxAllocates(t) {
		return t
	}
	return 0
}

// valueEntry charges a function value of fn: each call through it boxes what fn returns.
func (c *checker) valueEntry(n syntax.NodeID, fn EntityID, sig TypeID) {
	ret := c.r.Types.Node(c.vars.resolve(sig)).Elem
	if c.r.Types.ContainsVar(ret) {
		c.boxEdge(n, ret, whyEntryBox)
		return
	}
	if t := c.r.EntryBoxes(fn, ret); t != 0 {
		c.edge(EffectEdge{Kind: EdgeAlloc, Node: n, Type: t, Target: fn, Why: whyEntryBox})
	}
}

// boxedResult charges a call whose result arrives boxed from a callee only a value or a table names.
func (c *checker) boxedResult(n syntax.NodeID, ret TypeID, why uint8) {
	c.boxEdge(n, c.vars.resolve(ret), why)
}

func (c *checker) boxEdge(n syntax.NodeID, t TypeID, why uint8) {
	e := EffectEdge{Kind: EdgeAlloc, Node: n, Type: t, Why: why}
	if c.r.Types.ContainsVar(t) {
		c.edge(e)
		c.allocPending = append(c.allocPending, pendingAlloc{edges: c.edges, at: len(*c.edges) - 1})
		return
	}
	if c.r.boxCharged(why, t) {
		c.edge(e)
	}
}

// witnessArgs retags the crossings of a witness call's own arguments made since from.
func (c *checker) witnessArgs(from int, args []syntax.NodeID) {
	own := map[syntax.NodeID]bool{}
	for _, a := range args {
		own[a] = true
		own[c.t.View(a).ArgValue().ID()] = true
	}
	for i := from; i < len(*c.edges); i++ {
		if e := &(*c.edges)[i]; e.Kind == EdgeAlloc && e.Why == whyDirectCross && own[e.Node] {
			e.Why = whyWitnessCross
		}
	}
}

// requirementResult charges what a requirement call boxes; a witness call's charges wait for the built program.
func (c *checker) requirementResult(n syntax.NodeID, ret TypeID, from int, args []syntax.NodeID, witness, async bool) {
	switch {
	case async:
	case witness:
		c.witnessArgs(from, args)
		c.boxedResult(n, ret, whyWitnessResult)
	default:
		c.boxedResult(n, ret, whyDynamicResult)
	}
}
