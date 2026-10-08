package sem

import "kigumi/internal/syntax"

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

// A poisoned fn-typed field reaches the runner: a fn type has no witness pool to fall back on, unlike an interface.
func (r *Result) rootRunPoisonedFnFields(rr *rootRun) {
	for key := range r.fieldWitnessPoisoned {
		if !r.fnTypedFieldKey(key) {
			continue
		}
		if ent, ok := r.fieldParamEnts[key]; ok {
			rr.dist[ent] = 1
			rr.poisonedFnField[ent] = r.poisonedFieldHop(key)
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
			rr.dist[fwd.dst] = 1
			rr.poisonedFnField[fwd.dst] = r.poisonedFieldHop(fwd.src)
			rr.poisonAt[bindSite{fwd.file, fwd.node, fwd.dst}] = r.poisonedFieldHop(fwd.src)
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
				if e.Kind == EdgeFieldBind && e.Param == param && e.File == hops[i].File && e.Node == hops[i].Node &&
					!r.retractedBinds[bindSite{e.File, e.Node, e.Param}] && rr.hops(e.Target) > 0 {
					return e, true
				}
			}
		}
	}
	return EffectEdge{}, false
}
