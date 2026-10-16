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
		setDirect(s, FactAlloc, e, false)
	case EdgePanic:
		setDirect(s, FactPanic, e, false)
	case EdgeIO:
		setDirect(s, FactIO, e, false)
	case EdgeSuspend:
		setDirect(s, FactSuspend, e, false)
	case EdgeDrop:
		mergeInto(s, r.dropOpSummary(e.Type), e, false)
	case EdgeCall, EdgeWitness:
		if e.Target != 0 {
			mergeInto(s, r.opSummary(e.Target), e, false)
		}
	case EdgeCallback:
		r.applyCallbackOpEdge(s, e)
	case EdgeParamCall, EdgeWitnessCall:
		// Deferred to whichever call site resolves the parameter/witness
		// with a concrete argument; see OpSummary's Unresolved doc.
		s.Unresolved = true
	case EdgeAsm:
		r.applyAsmOpEdge(s, e)
	}
}

// applyCallbackOpEdge mirrors edgeAllows' EdgeCallback case. e.Offloaded (set only for an offloading
// provider's work argument, e.g. BlockingPool.run) forces every merged fact Offloaded.
func (r *Result) applyCallbackOpEdge(s *OpSummary, e EffectEdge) {
	if e.Param != 0 && !slices.Contains(r.Fn(r.Entities[e.Param].Parent).CalledParams, e.Param) {
		return
	}
	if e.Target != 0 {
		mergeInto(s, r.opSummary(e.Target), e, e.Offloaded)
		return
	}
	if e.Mods&EffNoalloc == 0 {
		setDirect(s, FactAlloc, e, e.Offloaded)
	}
	for f := OpFact(0); f < numOpFacts; f++ {
		if f != FactAlloc {
			setUnknown(s, f, e, e.Offloaded)
		}
	}
}

func setDirect(s *OpSummary, f OpFact, e EffectEdge, offloaded bool) {
	takeCause(&s.Facts[f], OpFactState{Maybe: true, Cause: OpCause{Edge: e}, Offloaded: offloaded})
}

func setUnknown(s *OpSummary, f OpFact, e EffectEdge, offloaded bool) {
	takeCause(&s.Facts[f], OpFactState{Maybe: true, Cause: OpCause{Edge: e, Unknown: true}, Offloaded: offloaded})
}

// mergeInto folds a callee/drop-glue summary into s under edge e, recording e as the cause; Unresolved
// propagates transitively, and offloaded forces every fact Offloaded regardless of from's own cause.
func mergeInto(s *OpSummary, from OpSummary, e EffectEdge, offloaded bool) {
	if from.Unresolved {
		s.Unresolved = true
	}
	s.Untrusted = s.Untrusted || from.Untrusted
	s.Contracts = unionContracts(s.Contracts, from.Contracts)
	for f := OpFact(0); f < numOpFacts; f++ {
		if st := from.Facts[f]; st.Maybe {
			takeCause(&s.Facts[f], OpFactState{Maybe: true, Cause: OpCause{Edge: e, Unknown: st.Cause.Unknown}, InterruptUnsafe: st.InterruptUnsafe, Offloaded: offloaded || st.Offloaded, Contract: st.Contract})
		}
	}
}

// causeRank orders a fact's possible causes (unset < contracted < unknown < offloaded < direct) so a genuine
// direct cause can never be shadowed by an offloaded or contracted one reached first, whatever the edge order.
func causeRank(s OpFactState) int {
	switch {
	case !s.Maybe:
		return 0
	case s.Cause.Unknown:
		return 2
	case s.Offloaded:
		return 3
	case s.Contract:
		return 1
	default:
		return 4
	}
}

// takeCause picks a fact's recorded cause by rank, not by arrival order;
// within the same rank, the first cause wins.
func takeCause(dst *OpFactState, next OpFactState) {
	if causeRank(next) > causeRank(*dst) {
		*dst = next
	}
}
