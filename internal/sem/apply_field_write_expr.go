package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

func (r *Result) applyFieldWriteExpr(file FileID, key fieldFnKey, expr syntax.NodeID, caller EntityID, call syntax.NodeID) bool {
	if r.fieldWitnessPoisoned[key] {
		return false
	}
	r.fieldParamEnt(key)
	switch r.Types.Kind(r.Entities[key.Field].Type) {
	case KFn, KClosure, KParam:
		return r.applyFnFieldWriteExpr(file, key, expr, caller, call)
	case KIface:
		return r.applyWitnessFieldWriteExpr(file, key, expr, caller, call)
	}
	return false
}

func (r *Result) applyFnFieldWriteExpr(file FileID, key fieldFnKey, expr syntax.NodeID, caller EntityID, call syntax.NodeID) bool {
	if fns := r.chainKnownFnRefs(file, expr); len(fns) > 0 {
		return r.joinKnownFieldFnAt(key, fns, fnSiteSrc{file: file, node: expr})
	}
	if ent := r.chainIdentEnt(file, expr); ent != 0 && r.Entities[ent].Kind == EntParam {
		return r.addFieldBindEdge(caller, EdgeFieldBind, ent, key, file, call)
	}
	return r.poisonFieldKeyAt(key, file, expr)
}

func (r *Result) applyWitnessFieldWriteExpr(file FileID, key fieldFnKey, expr syntax.NodeID, caller EntityID, call syntax.NodeID) bool {
	src := r.chainFieldEnt(file, expr)
	if src == 0 {
		return r.poisonFieldKeyAt(key, file, expr)
	}
	if r.Entities[src].Kind == EntLocal {
		if w, ok := r.localWitness[src]; ok {
			return r.joinLocalWitness(r.fieldParamEnt(key), w)
		}
		return r.poisonFieldKeyAt(key, file, expr)
	}
	return r.addFieldBindEdge(caller, EdgeIfaceArg, src, key, file, call)
}

func (r *Result) joinKnownFieldFn(key fieldFnKey, add []EntityID) bool {
	return r.joinKnownFieldFnAt(key, add, fnSiteSrc{})
}

// fieldBindOwners lets poisonFieldKeyGlobal retract an edge added before a later site poisons the key.
func (r *Result) addFieldBindEdge(fn EntityID, kind EdgeKind, target EntityID, key fieldFnKey, file FileID, node syntax.NodeID) bool {
	param := r.fieldParamEnt(key)
	info := r.Fn(fn)
	for _, e := range info.Edges {
		if e.Kind == kind && e.Target == target && e.Param == param {
			return false
		}
	}
	info.Edges = append(info.Edges, EffectEdge{Kind: kind, Target: target, Param: param, File: file, Node: node})
	r.noteFieldBindOwner(param, fn)
	return true
}

func (r *Result) noteFieldBindOwner(param, fn EntityID) {
	if r.fieldBindOwners == nil {
		r.fieldBindOwners = map[EntityID][]EntityID{}
	}
	if !slices.Contains(r.fieldBindOwners[param], fn) {
		r.fieldBindOwners[param] = append(r.fieldBindOwners[param], fn)
	}
}

func (r *Result) poisonFieldKeyAt(key fieldFnKey, file FileID, node syntax.NodeID) bool {
	here := fieldPoisonSiteInfo{File: file, Node: node}
	return r.poisonFieldKeyFrom(key, here, here)
}

// note is the write to cite; here is where it takes effect in the key's own body.
func (r *Result) poisonFieldKeyFrom(key fieldFnKey, note, here fieldPoisonSiteInfo) bool {
	keepEarlier(r, &r.fieldPoisonSite, key, note)
	keepEarlier(r, &r.flow.poisonAt, key, here)
	return r.poisonFieldKeyGlobal(key)
}

func (r *Result) poisonFieldKeyGlobal(key fieldFnKey) bool {
	if r.fieldWitnessPoisoned[key] {
		return false
	}
	delete(r.knownFieldFn, key)
	delete(r.fieldWitnessOf, key)
	if ent, ok := r.fieldParamEnts[key]; ok {
		delete(r.localWitness, ent)
		for _, fn := range r.fieldBindOwners[ent] {
			edges := &r.Fn(fn).Edges
			for _, e := range *edges {
				if e.Param == ent && (e.Kind == EdgeFieldBind || e.Kind == EdgeIfaceArg) {
					r.noteBindHist(ent, fn, e)
				}
			}
			*edges = slices.DeleteFunc(*edges, func(e EffectEdge) bool { return e.Param == ent })
		}
		delete(r.fieldBindOwners, ent)
	}
	r.fieldWitnessPoisoned[key] = true
	return true
}

func (r *Result) noteBindHist(ent, owner EntityID, e EffectEdge) {
	if r.flow.bindHist == nil {
		r.flow.bindHist = map[EntityID][]histBind{}
	}
	r.flow.bindHist[ent] = append(r.flow.bindHist[ent], histBind{owner: owner, edge: e})
}
