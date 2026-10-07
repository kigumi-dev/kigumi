package sem

func (r *Result) reportRootBridge(id EntityID, rr *rootRun) {
	hops := r.rootRunHops(id, rr)
	file, node := r.Entities[id].File, r.Entities[id].Node
	if len(hops) > 0 {
		file, node = hops[0].File, hops[0].Node
	}
	d, ok := r.diagAt(file, node, cRootBridgeInTask)
	if !ok {
		return
	}
	for i := 1; i < len(hops); i++ {
		d = d.WithNote(r.loc(hops[i].File, r.Files[hops[i].File].span(hops[i].Node)), hops[i].Text)
	}
	r.emitDiag(file, d)
}

// rootRunHops walks hop counts down to the runner; a subst-narrowed step isn't
// guaranteed to strictly decrease rr.dist, so visited bounds a forwarding cycle.
func (r *Result) rootRunHops(id EntityID, rr *rootRun) []OpHop {
	var hops []OpHop
	var subst map[EntityID][]EntityID
	visited := map[EntityID]bool{}
	for cur := id; rr.dist[cur] > 0; {
		if visited[cur] {
			break
		}
		visited[cur] = true
		if r.Entities[cur].Kind == EntParam || r.Entities[cur].Kind == EntFieldParam {
			e := rr.bind[cur]
			hops = append(hops, OpHop{e.File, e.Node, r.boundHopText(cur, e.Target)})
			cur, subst = e.Target, nil
			continue
		}
		if w, ok := rr.witness[cur]; ok && subst == nil {
			hops = append(hops, OpHop{r.Entities[w].File, r.Entities[w].Node, r.witnessHopText(cur, w)})
			cur = w
			continue
		}
		e, next, w, nextSubst := r.rootRunStepSubst(cur, subst, rr)
		if e == nil {
			break
		}
		if next == 0 {
			return append(hops, OpHop{e.File, e.Node, "drives a new root task here"})
		}
		hops = append(hops, OpHop{e.File, e.Node, "calls `" + r.Entities[next].Name + "`"})
		if w != 0 {
			hops = append(hops, OpHop{r.Entities[w].File, r.Entities[w].Node, r.witnessHopText(next, w)})
			cur, subst = w, nil
			continue
		}
		cur, subst = next, nextSubst
	}
	return hops
}

func (r *Result) witnessHopText(req, w EntityID) string {
	owner := r.Entities[r.Fn(w).Owner].Name
	return "`" + owner + "` conforms to `" + r.Entities[r.Entities[req].Parent].Name + "` here and may receive this call"
}

func (r *Result) boundHopText(param, arg EntityID) string {
	name := "`" + r.Entities[arg].Name + "`"
	if r.Entities[arg].Kind == EntClosure {
		name = "this closure"
	}
	return name + " is bound to `" + r.Entities[param].Name + "` here"
}

// rootRunStepSubst picks id's next hop: id's own flat distance with no
// subst, or the smallest hop count under an inherited one, as calleeHopsWith chose.
func (r *Result) rootRunStepSubst(id EntityID, subst map[EntityID][]EntityID, rr *rootRun) (e *EffectEdge, next, witness EntityID, nextSubst map[EntityID][]EntityID) {
	edges := r.edgesOf(id)
	best := 0
	for i := range edges {
		cand := &edges[i]
		if !rootRunCallEdge(*cand, rr) {
			continue
		}
		d, w, sub2 := r.edgeHopsSubst(id, *cand, subst, rr)
		if subst == nil {
			if d != rr.dist[id] {
				continue
			}
		} else if d == 0 || (best != 0 && d >= best) {
			continue
		}
		best = d
		e, witness, nextSubst = cand, w, sub2
		if cand.Target == rr.runner {
			next = 0
		} else {
			next = cand.Target
		}
		if subst == nil {
			return
		}
	}
	return
}

// edgeHopsSubst is edgeHops plus the witness it resolved to, preferring an
// inherited subst over the whole-program pool; nextSubst carries a fresh one forward.
func (r *Result) edgeHopsSubst(caller EntityID, e EffectEdge, subst map[EntityID][]EntityID, rr *rootRun) (hops int, witness EntityID, nextSubst map[EntityID][]EntityID) {
	switch e.Kind {
	case EdgeParamCall, EdgeCallback, EdgeKnownCall:
		if w, ok := subst[e.Target]; ok {
			d, _ := minHopsWitness(w, rr)
			return d, 0, nil
		}
		return rr.hops(e.Target), 0, nil
	case EdgeCall:
	default:
		return rr.hops(e.Target), 0, nil
	}
	if _, isReq := rr.impls[e.Target]; isReq {
		if w, ok := subst[e.Param]; ok {
			d, wit := minHopsWitness(w, rr)
			return d, wit, nil
		}
		if hd, wit, ok := r.ifaceCallWitness(e, rr); ok {
			return hd, wit, nil
		}
		return rr.hops(e.Target), 0, nil
	}
	sub2 := r.forwardSubst(caller, e, subst, rr)
	if len(sub2) == 0 {
		return rr.hops(e.Target), 0, nil
	}
	visited := map[EntityID]bool{caller: true, e.Target: true}
	if sd := r.calleeHopsWith(e.Target, sub2, rr, visited); sd > 0 {
		return sd + 1, 0, sub2
	}
	return 0, 0, nil
}
