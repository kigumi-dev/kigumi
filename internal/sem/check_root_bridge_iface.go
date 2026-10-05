package sem

import "slices"

// inferIfaceParamWitness grows rr.paramWitness to a fixpoint over
// EdgeIfaceArg edges, the union-set counterpart of rootRunBind's hop map.
func (r *Result) inferIfaceParamWitness(nodes []EntityID, rr *rootRun) bool {
	changed := false
	for _, id := range nodes {
		for _, e := range r.edgesOf(id) {
			if e.Kind != EdgeIfaceArg || e.Param == 0 {
				continue
			}
			w, ok := r.witnessOf(e.Target, rr)
			if !ok {
				continue
			}
			if r.unionWitness(rr, e.Param, w) {
				changed = true
			}
		}
	}
	return changed
}

func (r *Result) unionWitness(rr *rootRun, param EntityID, add []EntityID) bool {
	changed := false
	for _, w := range add {
		if !slices.Contains(rr.paramWitness[param], w) {
			rr.paramWitness[param] = append(rr.paramWitness[param], w)
			changed = true
		}
	}
	return changed
}

// witnessOf is an entity's own known witness set: an immutable local's
// comes from its let initializer, a parameter's from inferIfaceParamWitness.
func (r *Result) witnessOf(ent EntityID, rr *rootRun) ([]EntityID, bool) {
	if ent == 0 {
		return nil, false
	}
	switch r.Entities[ent].Kind {
	case EntLocal:
		w, ok := r.localWitness[ent]
		return w, ok
	case EntParam:
		w, ok := rr.paramWitness[ent]
		return w, ok
	}
	return nil, false
}

// ifaceCallHops narrows an EdgeCall into a requirement to the receiver's
// own witnesses; edgeHops falls back to rr.hops wherever ok is false.
func (r *Result) ifaceCallHops(e EffectEdge, rr *rootRun) (hops int, ok bool) {
	hops, _, ok = r.ifaceCallWitness(e, rr)
	return hops, ok
}

// ifaceCallWitness is ifaceCallHops plus the witness it narrowed to, so a
// report can cite that receiver instead of the whole-program pooled one.
func (r *Result) ifaceCallWitness(e EffectEdge, rr *rootRun) (hops int, witness EntityID, ok bool) {
	if _, isReq := rr.impls[e.Target]; !isReq {
		return 0, 0, false
	}
	w, known := r.witnessOf(e.Param, rr)
	if !known {
		return 0, 0, false
	}
	hops, witness = minHopsWitness(w, rr)
	return hops, witness, true
}

func minHops(witnesses []EntityID, rr *rootRun) int {
	best, _ := minHopsWitness(witnesses, rr)
	return best
}

func minHopsWitness(witnesses []EntityID, rr *rootRun) (int, EntityID) {
	best, via := 0, EntityID(0)
	for _, w := range witnesses {
		if d := rr.hops(w); d > 0 && (best == 0 || d < best) {
			best, via = d, w
		}
	}
	return best, via
}

// forwardSubst collects the interface-typed arguments caller's own edges
// know at e's call site, preferring an inherited subst over the global union.
func (r *Result) forwardSubst(caller EntityID, e EffectEdge, subst map[EntityID][]EntityID, rr *rootRun) map[EntityID][]EntityID {
	var out map[EntityID][]EntityID
	for _, ie := range r.edgesOf(caller) {
		if ie.Kind != EdgeIfaceArg || ie.Param == 0 || ie.File != e.File || ie.Node != e.Node {
			continue
		}
		w, ok := subst[ie.Target]
		if !ok {
			w, ok = r.witnessOf(ie.Target, rr)
		}
		if !ok {
			continue
		}
		if out == nil {
			out = map[EntityID][]EntityID{}
		}
		out[ie.Param] = w
	}
	return out
}

// calleeHopsWith is callee's distance to the runner, narrowing with subst
// along the whole forwarding chain under visited, not just callee's own calls.
func (r *Result) calleeHopsWith(callee EntityID, subst map[EntityID][]EntityID, rr *rootRun, visited map[EntityID]bool) int {
	best := 0
	for _, e := range r.edgesOf(callee) {
		if !rootRunCallEdge(e, rr) {
			continue
		}
		if d := r.forwardHops(callee, e, subst, rr, visited); d > 0 && (best == 0 || d < best) {
			best = d
		}
	}
	return best
}

// forwardHops is edgeHops' recursive step, narrowing via subst instead of
// the global witness pool.
func (r *Result) forwardHops(caller EntityID, e EffectEdge, subst map[EntityID][]EntityID, rr *rootRun, visited map[EntityID]bool) int {
	d := rr.hops(e.Target)
	if e.Kind != EdgeCall {
		return d
	}
	if _, isReq := rr.impls[e.Target]; isReq {
		if w, ok := subst[e.Param]; ok {
			return minHops(w, rr)
		}
		if hd, ok := r.ifaceCallHops(e, rr); ok {
			return hd
		}
		return d
	}
	// A cycle back to an ancestor contributes nothing new: its edges,
	// narrowed the same way, are already walked by the frame that visited it first.
	if visited[e.Target] {
		return 0
	}
	sub2 := r.forwardSubst(caller, e, subst, rr)
	if len(sub2) == 0 {
		var ok bool
		if sub2, ok = r.closureForwardSubst(e.Target, subst); !ok {
			return d
		}
	}
	visited2 := make(map[EntityID]bool, len(visited)+1)
	for id := range visited {
		visited2[id] = true
	}
	visited2[e.Target] = true
	if sd := r.calleeHopsWith(e.Target, sub2, rr, visited2); sd > 0 {
		return sd + 1
	}
	return 0
}

// edgeHops is forwardHops with no inherited subst and a fresh visited set.
func (r *Result) edgeHops(caller EntityID, e EffectEdge, rr *rootRun) int {
	return r.forwardHops(caller, e, nil, rr, map[EntityID]bool{caller: true})
}
