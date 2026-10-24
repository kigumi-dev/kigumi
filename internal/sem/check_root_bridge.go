package sem

import "kigumi/internal/syntax"

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

// A fn-typed parameter counts as a node here, standing for what call sites
// bind to it — and so does an interface requirement.
type rootRun struct {
	runner EntityID
	final  string
	dist   map[EntityID]int
	bind   map[EntityID]EffectEdge
	// invoked marks a parameter called directly or through a forwarding chain, not one only stored.
	invoked map[EntityID]bool
	impls   map[EntityID][]EntityID
	witness map[EntityID]EntityID
	// paramWitness narrows a requirement's witness pool to the receiver reaching this parameter.
	paramWitness map[EntityID][]EntityID
	// openParam is bound at some call site to a value with no known witnesses, so pooled witnesses can't narrow it.
	openParam  map[EntityID]bool
	ifaceBound map[EntityID]bool
	// poisonedFnField stands in for a bound argument since no caller ever supplied one.
	poisonedFnField map[EntityID]OpHop
	poisonAt        map[bindSite]OpHop
	// unresolved stands for a call site's binding of a poisoned source, so only that site reaches the runner.
	unresolved     EntityID
	poisonBySite   map[siteLoc][]EntityID
	poisonedPlaces map[EntityID]bool
	siteEdges      map[EntityID]map[siteLoc][]EffectEdge
}

type siteLoc struct {
	file FileID
	node syntax.NodeID
}

// A dynamic target proves nothing, so an unresolved call is not a bridge.
func (rr *rootRun) hops(target EntityID) int {
	switch {
	case target == 0:
		return 0
	case target == rr.runner || target == rr.unresolved:
		return 1
	case rr.dist[target] > 0:
		return rr.dist[target] + 1
	}
	return 0
}

// inferRootRun is checkInterruptHandlers' fixpoint shape over call edges.
func (r *Result) inferRootRun(nodes []EntityID, runner EntityID) *rootRun {
	return r.inferRunnerReach(nodes, runner, "drives a new root task here", false)
}

// syncOnly skips async bodies, which run inside a task and never on their caller's thread.
func (r *Result) inferRunnerReach(nodes []EntityID, runner EntityID, final string, syncOnly bool) *rootRun {
	rr := &rootRun{runner: runner, final: final, dist: map[EntityID]int{}, bind: map[EntityID]EffectEdge{}, invoked: map[EntityID]bool{},
		impls: r.ifaceReqImpls(), witness: map[EntityID]EntityID{}, paramWitness: map[EntityID][]EntityID{}, openParam: map[EntityID]bool{},
		poisonedFnField: map[EntityID]OpHop{}, poisonAt: map[bindSite]OpHop{}}
	if r.runnerDriven(nodes, runner) {
		r.rootRunPoisonedFnFields(rr)
	}
	for changed := true; changed; {
		changed = false
		if r.rootRunResolveInvoked(nodes, rr) {
			changed = true
		}
		for req, impls := range rr.impls {
			if r.rootRunImpl(req, impls, rr) {
				changed = true
			}
		}
		if r.inferIfaceParamWitness(nodes, rr) {
			changed = true
		}
		for _, id := range nodes {
			if r.rootRunBind(id, rr) {
				changed = true
			}
			if r.rootRunFieldBind(id, rr) {
				changed = true
			}
			if syncOnly && r.Entities[id].Kind == EntFn && r.Fn(id).Declared&EffAsync != 0 {
				continue
			}
			if d := r.rootRunDist(id, rr); d > 0 && (rr.dist[id] == 0 || d < rr.dist[id]) {
				rr.dist[id], changed = d, true
			}
		}
	}
	return rr
}

// rootRunResolveInvoked marks forwarded parameters called directly or through another invoked one.
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

// rootRunFieldBind is rootRunBind's counterpart for a stored field: no
// invoked gate, since an EntFieldParam only exists for a field actually called.
func (r *Result) rootRunFieldBind(id EntityID, rr *rootRun) bool {
	changed := false
	for _, e := range r.edgesOf(id) {
		if e.Kind != EdgeFieldBind || e.Param == 0 || rr.poisonedPlace(r, e.Target) {
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
		if !rootRunCallEdge(e, rr) {
			continue
		}
		if d := r.edgeHops(id, e, rr); d > 0 && (best == 0 || d < best) {
			best = d
		}
	}
	return best
}

// rootRunCallEdge: EdgeCall/EdgeWitness/EdgeKnownCall always count, since a
// resolved callee has no deferred param to gate on; EdgeCallback/EdgeParamCall need e.Param invoked (or e.Param == 0, meaning unknown body or already-invocation).
func rootRunCallEdge(e EffectEdge, rr *rootRun) bool {
	switch e.Kind {
	case EdgeCall, EdgeWitness, EdgeKnownCall:
		return true
	case EdgeCallback, EdgeParamCall:
		return e.Param == 0 || rr.invoked[e.Param]
	}
	return false
}
