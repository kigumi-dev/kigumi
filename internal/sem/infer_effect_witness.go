package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

// WitnessReqKey names a `pure?` interface requirement read through one of
// a function's own type parameters for generic instantiation.
type WitnessReqKey struct {
	Param EntityID
	Req   EntityID
}

func (c *checker) deferToWitnessReq(param, req EntityID) {
	c.r.addCalledWitnessReq(c.fn, param, req)
}

func (r *Result) addCalledWitnessReq(fn, param, req EntityID) bool {
	info := r.Fn(fn)
	key := WitnessReqKey{Param: param, Req: req}
	if slices.Contains(info.CalledWitnessReqs, key) {
		return false
	}
	info.CalledWitnessReqs = append(info.CalledWitnessReqs, key)
	return true
}

// pendingWitnessCallback exists because reading CalledWitnessReqs mid-pass
// sees it empty when fn is checked after the call site, so this removes the
// order dependency instead of chasing every receiver shape.
type pendingWitnessCallback struct {
	owner  EntityID
	fn     EntityID
	node   syntax.NodeID
	file   FileID
	scoped bool
	passes []WitnessArg
}

func (c *checker) deferWitnessCallback(fn EntityID, node syntax.NodeID, passes []WitnessArg) {
	owner := c.fn
	if n := len(c.lambdas); n > 0 {
		owner = c.lambdas[n-1].ent
	}
	c.r.pendingWitnessCallbacks = append(c.r.pendingWitnessCallbacks, pendingWitnessCallback{
		owner: owner, fn: fn, node: node, file: c.f, scoped: c.allocDepth > 0, passes: passes,
	})
}

func (r *Result) resolvePendingWitnessCallbacks() {
	pending := r.pendingWitnessCallbacks
	r.pendingWitnessCallbacks = nil
	r.recoverable.sites = append(r.recoverable.sites, pending...)
	for _, p := range pending {
		r.witnessCallbackEdges(p)
	}
}

// witnessCallbackEdges: a witness still forwarded from an outer type
// parameter (WitnessLocal) is not chased further and stays the conservative
// default of impure, like an interface value's dynamic dispatch.
func (r *Result) witnessCallbackEdges(p pendingWitnessCallback) {
	if bound := boundWitness(p.passes); bound != 0 && !r.boundAsValue(p) {
		r.addEdge(p.owner, EffectEdge{Kind: EdgeAlloc, Target: bound, Why: whyWitnessBind, Node: p.node, File: p.file, Scoped: p.scoped})
		if r.lendSites == nil {
			r.lendSites = map[lendSite]lendSite{}
		}
		r.lendSites[lendSite{p.file, p.node}] = lendSite{p.file, p.node}
	}
	info := r.Fn(p.fn)
	for _, key := range info.CalledWitnessReqs {
		i := slices.IndexFunc(info.Witnesses, func(w WitnessSlot) bool {
			return w.Param == key.Param && w.Req == key.Req
		})
		if i < 0 || i >= len(p.passes) {
			continue
		}
		r.addEdge(p.owner, EffectEdge{Kind: EdgeCallback, Mods: r.witnessArgMods(p.passes[i]), Node: p.node, File: p.file, Scoped: p.scoped})
	}
}

// addEdge appends straight to owner's edge list, after checkBodies has
// already copied it in from the checker's local slice (see checkFnBody).
func (r *Result) addEdge(owner EntityID, e EffectEdge) {
	if r.Entities[owner].Kind == EntClosure {
		info := r.closure(owner)
		info.Edges = append(info.Edges, e)
		return
	}
	info := r.Fn(owner)
	info.Edges = append(info.Edges, e)
}

// witnessArgMods checks a WitnessBind's own CalledWitnessReqs recursively:
// its declared purity only holds once those also resolve purely
// against the bound Args.
func (r *Result) witnessArgMods(arg WitnessArg) Effects {
	if arg.Ent == 0 || arg.Kind == WitnessLocal {
		return 0
	}
	mods := r.Fn(arg.Ent).Declared & (EffPure | EffNoalloc)
	if arg.Kind != WitnessBind {
		return mods
	}
	info := r.Fn(arg.Ent)
	for _, key := range info.CalledWitnessReqs {
		i := slices.IndexFunc(info.Witnesses, func(w WitnessSlot) bool {
			return w.Param == key.Param && w.Req == key.Req
		})
		if i < 0 || i >= len(arg.Args) {
			return 0
		}
		mods &= r.witnessArgMods(arg.Args[i])
	}
	return mods
}

// whyWitnessBind: handing a generic impl over as a witness binds its own witnesses into a closure.
const whyWitnessBind = 24

func boundWitness(passes []WitnessArg) EntityID {
	for _, a := range passes {
		if a.Kind == WitnessBind {
			return a.Ent
		}
	}
	return 0
}

// boundAsValue: a function value already pays for its bind (whyBind).
func (r *Result) boundAsValue(p pendingWitnessCallback) bool {
	for _, e := range r.edgesOf(p.owner) {
		if e.Kind == EdgeAlloc && e.Why == whyBind && e.Node == p.node && e.File == p.file {
			return true
		}
	}
	return false
}
