package sem

import "slices"

// inferOpSummaries computes every node's OpSummary to a fixpoint, the same
// monotone-least-fixpoint shape as inferSummaries.
func (r *Result) inferOpSummaries(nodes []EntityID) {
	var inferred []EntityID
	for _, id := range nodes {
		if _, fixed := r.opFixedSummary(id); !fixed {
			inferred = append(inferred, id)
		}
	}
	for changed := true; changed; {
		changed = false
		for _, id := range inferred {
			next := r.computeOpSummary(id)
			cur := r.opSummaryPtr(id)
			// Full equality, not just Maybe: a cause can still refine after Maybe itself stops changing.
			if *cur != next {
				*cur, changed = next, true
			}
		}
	}
}

// computeOpSummary rebuilds id's summary from its own fixed edge list each
// pass, keeping the recorded first cause independent of iteration order.
func (r *Result) computeOpSummary(id EntityID) OpSummary {
	s := OpSummary{Known: true}
	if r.Entities[id].Kind == EntFn {
		info := r.Fn(id)
		if len(info.CalledParams) > 0 || len(info.CalledFieldParams) > 0 || len(info.CalledWitnessReqs) > 0 {
			s.Unresolved = true
		}
	}
	for _, e := range r.edgesOf(id) {
		r.applyOpEdge(&s, e)
	}
	return s
}

func (r *Result) applyOpEdge(s *OpSummary, e EffectEdge) {
	switch e.Kind {
	case EdgeAlloc:
		setDirect(s, FactAlloc, e)
	case EdgePanic:
		setDirect(s, FactPanic, e)
	case EdgeIO:
		setDirect(s, FactIO, e)
	case EdgeSuspend:
		setDirect(s, FactSuspend, e)
	case EdgeDrop:
		mergeInto(s, r.dropOpSummary(e.Type), e)
	case EdgeCall, EdgeWitness:
		if e.Target != 0 {
			mergeInto(s, r.opSummary(e.Target), e)
		}
	case EdgeCallback:
		r.applyCallbackOpEdge(s, e)
	case EdgeParamCall, EdgeWitnessCall:
		// Deferred to whichever call site resolves the parameter/witness
		// with a concrete argument; see OpSummary's Unresolved doc.
		s.Unresolved = true
	}
}

// applyCallbackOpEdge mirrors edgeAllows' EdgeCallback case: a plain
// fn-typed value only bounds FactAlloc, the one fact function types carry.
func (r *Result) applyCallbackOpEdge(s *OpSummary, e EffectEdge) {
	if e.Param != 0 && !slices.Contains(r.Fn(r.Entities[e.Param].Parent).CalledParams, e.Param) {
		return
	}
	if e.Target != 0 {
		mergeInto(s, r.opSummary(e.Target), e)
		return
	}
	if e.Mods&EffNoalloc == 0 {
		setDirect(s, FactAlloc, e)
	}
	for f := OpFact(0); f < numOpFacts; f++ {
		if f != FactAlloc {
			setUnknown(s, f, e)
		}
	}
}

func setDirect(s *OpSummary, f OpFact, e EffectEdge) {
	takeCause(&s.Facts[f], OpFactState{Maybe: true, Cause: OpCause{Edge: e}})
}

func setUnknown(s *OpSummary, f OpFact, e EffectEdge) {
	takeCause(&s.Facts[f], OpFactState{Maybe: true, Cause: OpCause{Edge: e, Unknown: true}})
}

// mergeInto folds a callee/drop-glue summary into s under edge e, recording
// e itself as the cause; Unresolved propagates transitively.
func mergeInto(s *OpSummary, from OpSummary, e EffectEdge) {
	if from.Unresolved {
		s.Unresolved = true
	}
	for f := OpFact(0); f < numOpFacts; f++ {
		if from.Facts[f].Maybe {
			takeCause(&s.Facts[f], OpFactState{Maybe: true, Cause: OpCause{Edge: e, Unknown: from.Facts[f].Cause.Unknown}, InterruptUnsafe: from.Facts[f].InterruptUnsafe})
		}
	}
}

// takeCause picks a fact's recorded first cause: a concrete edge always
// beats an unknown one; within the same concreteness, the earlier wins.
func takeCause(dst *OpFactState, next OpFactState) {
	switch {
	case !dst.Maybe:
		*dst = next
	case dst.Cause.Unknown && !next.Cause.Unknown:
		*dst = next
	}
}
