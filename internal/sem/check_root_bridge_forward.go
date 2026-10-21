package sem

// forwardSubst collects what caller's edges know at e's call site about each
// parameter: EdgeIfaceArg's witnesses, EdgeFnBoundArg's known fn, EdgeFieldBind's field fn.
func (r *Result) forwardSubst(caller EntityID, e EffectEdge, subst map[EntityID][]EntityID, rr *rootRun) map[EntityID][]EntityID {
	var out map[EntityID][]EntityID
	var open map[EntityID]bool
	for _, ie := range rr.edgesAt(r, caller, siteLoc{e.File, e.Node}) {
		var w []EntityID
		var ok bool
		switch ie.Kind {
		case EdgeIfaceArg:
			w, ok = subst[ie.Target]
			if !ok {
				w, ok = r.witnessOf(ie.Target, rr)
			}
		case EdgeFnBoundArg:
			w, ok = subst[ie.Target]
			if !ok && (r.Entities[ie.Target].Kind == EntFn || r.Entities[ie.Target].Kind == EntClosure) {
				w, ok = []EntityID{ie.Target}, true
			}
		case EdgeFieldBind:
			if r.retractedBinds[bindSite{ie.File, ie.Node, ie.Param}] {
				break
			}
			if rr.poisonedPlace(r, ie.Target) {
				w, ok = []EntityID{rr.unresolved}, true
				break
			}
			w, ok = subst[ie.Target]
			if !ok && (r.Entities[ie.Target].Kind == EntFn || r.Entities[ie.Target].Kind == EntClosure) {
				w, ok = []EntityID{ie.Target}, true
			}
			if !ok {
				w, ok = r.placeWriters(caller, ie.Target, subst)
			}
		default:
			continue
		}
		if !ok {
			if ie.Kind == EdgeFieldBind && !r.unboundPlace(ie.Target) {
				if open == nil {
					open = map[EntityID]bool{}
				}
				open[ie.Param] = true
			}
			continue
		}
		if out == nil {
			out = map[EntityID][]EntityID{}
		}
		if ie.Kind == EdgeFieldBind || ie.Kind == EdgeFnBoundArg {
			out[ie.Param] = append(out[ie.Param], w...)
		} else {
			out[ie.Param] = w
		}
	}
	for param := range open {
		delete(out, param)
	}
	for _, param := range rr.poisonBySite[siteLoc{e.File, e.Node}] {
		if out == nil {
			out = map[EntityID][]EntityID{}
		}
		out[param] = []EntityID{rr.unresolved}
	}
	return out
}

func (rr *rootRun) edgesAt(r *Result, caller EntityID, site siteLoc) []EffectEdge {
	idx, ok := rr.siteEdges[caller]
	if !ok {
		idx = map[siteLoc][]EffectEdge{}
		for _, ie := range r.edgesOf(caller) {
			if ie.Param != 0 {
				idx[siteLoc{ie.File, ie.Node}] = append(idx[siteLoc{ie.File, ie.Node}], ie)
			}
		}
		if rr.siteEdges == nil {
			rr.siteEdges = map[EntityID]map[siteLoc][]EffectEdge{}
		}
		rr.siteEdges[caller] = idx
	}
	return idx[site]
}

func (r *Result) unboundPlace(ent EntityID) bool {
	return r.Entities[ent].Kind == EntFieldParam && r.Entities[ent].Parent != 0 && r.Entities[r.rootOf(ent)].Kind != EntParam && len(r.fieldBindOwners[ent]) == 0
}

func (r *Result) rootOf(ent EntityID) EntityID {
	for r.Entities[ent].Kind == EntFieldParam && r.Entities[ent].Parent != 0 {
		ent = r.Entities[ent].Parent
	}
	return ent
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

// forwardHops is edgeHops' recursive step, narrowing via subst instead of the
// global witness pool; EdgeParamCall/EdgeCallback/EdgeKnownCall key on Target, not EdgeCall's Param.
func (r *Result) forwardHops(caller EntityID, e EffectEdge, subst map[EntityID][]EntityID, rr *rootRun, visited map[EntityID]bool) int {
	d := rr.hops(e.Target)
	switch e.Kind {
	case EdgeParamCall, EdgeCallback, EdgeKnownCall:
		if w, ok := subst[e.Target]; ok {
			return minHops(w, rr)
		}
		return d
	case EdgeCall:
	default:
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

func (r *Result) placeWriters(caller, place EntityID, subst map[EntityID][]EntityID) ([]EntityID, bool) {
	if r.Entities[place].Kind != EntFieldParam || len(r.fieldBindOwners[place]) == 0 {
		return nil, false
	}
	var out []EntityID
	for _, we := range r.edgesOf(caller) {
		if we.Kind != EdgeFieldBind || we.Param != place {
			continue
		}
		if w, ok := subst[we.Target]; ok {
			out = append(out, w...)
			continue
		}
		if k := r.Entities[we.Target].Kind; k != EntFn && k != EntClosure {
			return nil, false
		}
		out = append(out, we.Target)
	}
	return out, len(out) > 0
}
