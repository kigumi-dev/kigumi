package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// fieldPoisonForward is resolved only after every body is checked, once poisoning is final.
type fieldPoisonForward struct {
	src  fieldFnKey
	dst  EntityID
	file FileID
	node syntax.NodeID
}

type bindSite struct {
	file  FileID
	node  syntax.NodeID
	param EntityID
}

// retractPoisonedBindings makes a poisoned field's per-site binding order-independent; forwardSubst must not narrow with it.
func (r *Result) retractPoisonedBindings() {
	for _, fwd := range r.fieldPoisonForwards {
		if !r.fieldWitnessPoisoned[fwd.src] {
			continue
		}
		if r.retractedBinds == nil {
			r.retractedBinds = map[bindSite]bool{}
		}
		r.retractedBinds[bindSite{fwd.file, fwd.node, fwd.dst}] = true
	}
}

// runnerDriven reports whether any fn or closure names the runner, so a poisoned field could hold one.
func (r *Result) runnerDriven(nodes []EntityID, runner EntityID) bool {
	for _, id := range nodes {
		if k := r.Entities[id].Kind; k != EntFn && k != EntClosure {
			continue
		}
		for _, e := range r.edgesOf(id) {
			if e.Target == runner {
				return true
			}
		}
	}
	return false
}

// A poisoned fn-typed field reaches the runner: a fn type has no witness pool to fall back on, unlike an interface.
func (r *Result) rootRunPoisonedFnFields(rr *rootRun) {
	for key := range r.fieldWitnessPoisoned {
		if !r.fnTypedFieldKey(key) {
			continue
		}
		if ent, ok := r.fieldParamEnts[key]; ok {
			if rr.unresolved == 0 {
				rr.unresolved = r.unresolvedEntity()
			}
			rr.dist[ent] = 1
			rr.poisonedFnField[ent] = r.poisonedFieldHop(key)
			if rr.poisonedPlaces == nil {
				rr.poisonedPlaces = map[EntityID]bool{}
			}
			rr.poisonedPlaces[ent] = true
		}
	}
	for _, late := range r.lateFieldEdges {
		if late.kind == EdgeKnownCall && r.fieldWitnessPoisoned[late.src] && r.fnTypedFieldKey(late.src) && rr.dist[late.caller] == 0 {
			rr.dist[late.caller] = 1
			rr.poisonedFnField[late.caller] = r.poisonedFieldHop(late.src)
		}
	}
	for _, fwd := range r.fieldPoisonForwards {
		if r.fieldWitnessPoisoned[fwd.src] && r.fnTypedFieldKey(fwd.src) {
			rr.markPoisonedBind(r, fwd)
		}
	}
}

func (r *Result) poisonedFieldHop(key fieldFnKey) OpHop {
	name := "`" + r.Entities[key.Field].Name + "` was written to an unresolved value here, and could hold any fn"
	if site, ok := r.fieldPoisonSite[key]; ok {
		return OpHop{site.File, site.Node, name}
	}
	return OpHop{r.Entities[key.Field].File, r.Entities[key.Field].Node, name}
}

func (r *Result) fnTypedFieldKey(key fieldFnKey) bool {
	switch r.Types.Kind(r.Entities[key.Field].Type) {
	case KFn, KClosure, KParam:
		return true
	}
	return false
}

func (rr *rootRun) poisonHopAtSite(param EntityID, hops []OpHop) (OpHop, bool) {
	for i := len(hops) - 1; i >= 0; i-- {
		if hop, ok := rr.poisonAt[bindSite{hops[i].File, hops[i].Node, param}]; ok {
			return hop, true
		}
	}
	return OpHop{}, false
}

func (r *Result) siteBinding(param EntityID, hops []OpHop, rr *rootRun) (EffectEdge, bool) {
	nodes := r.effectNodes()
	for i := len(hops) - 1; i >= 0; i-- {
		for _, id := range nodes {
			for _, e := range r.edgesOf(id) {
				if (e.Kind == EdgeFieldBind || e.Kind == EdgeCallback) && e.Param == param && e.File == hops[i].File && e.Node == hops[i].Node &&
					!r.retractedBinds[bindSite{e.File, e.Node, e.Param}] && (rr.hops(e.Target) > 0 || rr.poisonedFnField[e.Target] != (OpHop{})) {
					return e, true
				}
			}
		}
	}
	return EffectEdge{}, false
}

// A poisoned source taints the call that passes it, not a sibling call into the same callee parameter.
func (rr *rootRun) markPoisonedBind(r *Result, fwd fieldPoisonForward) {
	if rr.unresolved == 0 {
		rr.unresolved = r.unresolvedEntity()
	}
	hop := r.poisonedFieldHop(fwd.src)
	rr.poisonedFnField[fwd.dst] = hop
	rr.poisonAt[bindSite{fwd.file, fwd.node, fwd.dst}] = hop
	if rr.poisonBySite == nil {
		rr.poisonBySite = map[siteLoc][]EntityID{}
	}
	loc := siteLoc{fwd.file, fwd.node}
	if !slices.Contains(rr.poisonBySite[loc], fwd.dst) {
		rr.poisonBySite[loc] = append(rr.poisonBySite[loc], fwd.dst)
	}
}

func (r *Result) unresolvedEntity() EntityID {
	if r.flow.unresolved == 0 {
		r.flow.unresolved = r.newEntity(Entity{Kind: EntFieldParam, Name: "<unresolved>"})
	}
	return r.flow.unresolved
}

func (r *Result) walkable(cur EntityID, subst map[EntityID][]EntityID, hops []OpHop, rr *rootRun) bool {
	if rr.dist[cur] > 0 || subst != nil || rr.poisonedFnField[cur] != (OpHop{}) {
		return true
	}
	if k := r.Entities[cur].Kind; k != EntParam && k != EntFieldParam {
		return false
	}
	_, ok := r.siteBinding(cur, hops, rr)
	return ok
}

func (rr *rootRun) poisonedPlace(r *Result, ent EntityID) bool {
	return rr.poisonedPlaces[ent] && r.Entities[ent].Kind == EntFieldParam
}
