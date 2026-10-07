package sem

import "slices"

// inferIfaceParamWitness grows rr.paramWitness to a fixpoint over
// EdgeIfaceArg edges, the union-set counterpart of rootRunBind's hop map.
func (r *Result) inferIfaceParamWitness(nodes []EntityID, rr *rootRun) bool {
	changed := false
	if rr.ifaceBound == nil {
		rr.ifaceBound = map[EntityID]bool{}
		for _, id := range nodes {
			for _, e := range r.edgesOf(id) {
				if e.Kind == EdgeIfaceArg && e.Param != 0 {
					rr.ifaceBound[e.Param] = true
				}
			}
		}
	}
	for _, id := range nodes {
		for _, e := range r.edgesOf(id) {
			if e.Kind != EdgeIfaceArg || e.Param == 0 {
				continue
			}
			w, ok := r.witnessOf(e.Target, rr)
			if !ok {
				if r.witnessUnknown(e.Target, rr) && !rr.openParam[e.Param] {
					rr.openParam[e.Param] = true
					changed = true
				}
				continue
			}
			if r.unionWitness(rr, e.Param, w) {
				changed = true
			}
		}
	}
	return changed
}

// witnessUnknown can never name witnesses, unlike a source the fixpoint hasn't reached; binding it opens the parameter to its whole pool.
func (r *Result) witnessUnknown(ent EntityID, rr *rootRun) bool {
	if ent == 0 {
		return true
	}
	switch r.Entities[ent].Kind {
	case EntLocal:
		_, known := r.localWitness[ent]
		return !known
	case EntParam, EntFieldParam:
		if rr.openParam[ent] {
			return true
		}
		_, known := r.localWitness[ent]
		return !known && !rr.ifaceBound[ent]
	}
	return true
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

// witnessOf is an entity's own known witness set: a local's from its let
// initializer, a param's from inferIfaceParamWitness, a field-param's both (unioned).
func (r *Result) witnessOf(ent EntityID, rr *rootRun) ([]EntityID, bool) {
	if ent == 0 {
		return nil, false
	}
	switch r.Entities[ent].Kind {
	case EntLocal:
		w, ok := r.localWitness[ent]
		return w, ok
	case EntParam:
		if rr.openParam[ent] {
			return nil, false
		}
		w, ok := rr.paramWitness[ent]
		return w, ok
	case EntFieldParam:
		if rr.openParam[ent] {
			return nil, false
		}
		lw, lok := r.localWitness[ent]
		pw, pok := rr.paramWitness[ent]
		if !lok && !pok {
			return nil, false
		}
		return append(append([]EntityID{}, lw...), pw...), true
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
