package sem

// Driving a root task is unsafe wherever a task is already running, not just
// in the body that lexically holds the call, so the reach is transitive.

func (r *Result) checkRootBridges() {
	runner := r.rootRunner()
	if runner == 0 {
		return
	}
	nodes := r.effectNodes()
	rr := r.inferRootRun(nodes, runner)
	for _, id := range nodes {
		if r.Entities[id].Kind == EntFn && r.Fn(id).Declared&EffAsync != 0 && rr.dist[id] > 0 {
			r.reportRootBridge(id, rr)
		}
	}
}

func (r *Result) rootRunner() EntityID {
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Kind == EntFn && e.Name == "runToCompletion" && r.Packages[e.Pkg].Path == "std/task" {
			return EntityID(id)
		}
	}
	return 0
}

// rootRun holds hop counts to the runner, keeping the witness walk finite.
// A fn-typed parameter is a node too, standing for what call sites bind to it.
type rootRun struct {
	runner EntityID
	dist   map[EntityID]int
	bind   map[EntityID]EffectEdge
	// invoked marks a parameter actually invoked, directly or through a
	// forwarding chain that bottoms out in an invocation; a parameter merely
	// stored or returned is never flagged even if a root-driving fn reaches it.
	invoked map[EntityID]bool
}

// A dynamic target proves nothing, so an unresolved call is not a bridge.
func (rr *rootRun) hops(target EntityID) int {
	switch {
	case target == 0:
		return 0
	case target == rr.runner:
		return 1
	case rr.dist[target] > 0:
		return rr.dist[target] + 1
	}
	return 0
}

// inferRootRun is checkInterruptHandlers' fixpoint shape over call edges.
func (r *Result) inferRootRun(nodes []EntityID, runner EntityID) *rootRun {
	rr := &rootRun{runner: runner, dist: map[EntityID]int{}, bind: map[EntityID]EffectEdge{}, invoked: map[EntityID]bool{}}
	for changed := true; changed; {
		changed = false
		if r.rootRunResolveInvoked(nodes, rr) {
			changed = true
		}
		for _, id := range nodes {
			if r.rootRunBind(id, rr) {
				changed = true
			}
			if d := r.rootRunDist(id, rr); d > 0 && (rr.dist[id] == 0 || d < rr.dist[id]) {
				rr.dist[id], changed = d, true
			}
		}
	}
	return rr
}

// rootRunResolveInvoked finds which forwarded parameters are actually called:
// an EdgeParamCall with Param == 0 is a direct invocation, and one forwarding
// into an already-invoked parameter is invoked transitively.
func (r *Result) rootRunResolveInvoked(nodes []EntityID, rr *rootRun) bool {
	changed := false
	for _, id := range nodes {
		for _, e := range r.edgesOf(id) {
			if e.Kind != EdgeParamCall || rr.invoked[e.Target] {
				continue
			}
			if e.Param == 0 || rr.invoked[e.Param] {
				rr.invoked[e.Target] = true
				changed = true
			}
		}
	}
	return changed
}

// rootRunBind resolves a deferred parameter call the way the effect system
// does, by letting the argument a call site supplies stand for the parameter.
func (r *Result) rootRunBind(id EntityID, rr *rootRun) bool {
	changed := false
	for _, e := range r.edgesOf(id) {
		if e.Kind != EdgeCallback || e.Param == 0 || !rr.invoked[e.Param] {
			continue
		}
		if d := rr.hops(e.Target); d > 0 && (rr.dist[e.Param] == 0 || d < rr.dist[e.Param]) {
			rr.dist[e.Param], rr.bind[e.Param] = d, e
			changed = true
		}
	}
	return changed
}

func (r *Result) rootRunDist(id EntityID, rr *rootRun) int {
	best := 0
	for _, e := range r.edgesOf(id) {
		if !rr.callEdgeCounts(e) {
			continue
		}
		if d := rr.hops(e.Target); d > 0 && (best == 0 || d < best) {
			best = d
		}
	}
	return best
}

// callEdgeCounts keeps EdgeCall/EdgeWitness unconditional (a resolved callee
// has no deferred parameter to gate on); EdgeCallback and EdgeParamCall only
// count once the parameter they carry (e.Param) is itself invoked, or when
// e.Param is 0, meaning the body is unknown or this already is the invocation.
func (rr *rootRun) callEdgeCounts(e EffectEdge) bool {
	switch e.Kind {
	case EdgeCall, EdgeWitness:
		return true
	case EdgeCallback, EdgeParamCall:
		return e.Param == 0 || rr.invoked[e.Param]
	}
	return false
}

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

// rootRunHops walks strictly decreasing hop counts, so one witness path
// always exists and no cycle can be entered.
func (r *Result) rootRunHops(id EntityID, rr *rootRun) []OpHop {
	var hops []OpHop
	for cur := id; rr.dist[cur] > 0; {
		if r.Entities[cur].Kind == EntParam {
			e := rr.bind[cur]
			hops = append(hops, OpHop{e.File, e.Node, r.boundHopText(cur, e.Target)})
			cur = e.Target
			continue
		}
		e, next := r.rootRunStep(cur, rr)
		if e == nil {
			break
		}
		if next == 0 {
			return append(hops, OpHop{e.File, e.Node, "drives a new root task here"})
		}
		hops = append(hops, OpHop{e.File, e.Node, "calls `" + r.Entities[next].Name + "`"})
		cur = next
	}
	return hops
}

func (r *Result) boundHopText(param, arg EntityID) string {
	name := "`" + r.Entities[arg].Name + "`"
	if r.Entities[arg].Kind == EntClosure {
		name = "this closure"
	}
	return name + " is bound to `" + r.Entities[param].Name + "` here"
}

func (r *Result) rootRunStep(id EntityID, rr *rootRun) (*EffectEdge, EntityID) {
	edges := r.edgesOf(id)
	for i := range edges {
		e := &edges[i]
		if !rr.callEdgeCounts(*e) || rr.hops(e.Target) != rr.dist[id] {
			continue
		}
		if e.Target == rr.runner {
			return e, 0
		}
		return e, e.Target
	}
	return nil, 0
}
