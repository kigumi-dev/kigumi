package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

// methodFieldCallSite is a call to a known `mut self` method, staged during
// body-checking since the callee's own field-write summary may not exist yet.
type methodFieldCallSite struct {
	file   FileID
	node   syntax.NodeID
	caller EntityID
	recv   EntityID
	method EntityID
	args   []syntax.NodeID
}

// recordMethodFieldCallSite stages recv.method(args) for closeMethodFieldWrites,
// resolving a nested-field receiver (`o.inner.setC(...)`) the same way a read does.
func (c *checker) recordMethodFieldCallSite(n syntax.NodeID, m EntityID, base syntax.NodeID, args []syntax.NodeID) {
	recv := c.r.chainFieldEnt(c.f, base)
	if recv == 0 {
		return
	}
	site := methodFieldCallSite{file: c.f, node: n, caller: c.fn, recv: recv, method: m, args: args}
	c.r.methodFieldCalls = append(c.r.methodFieldCalls, site)
	// A sibling statement right after this one may need the result now; the
	// fixpoint below is the one that redoes it soundly, for every ordering.
	c.r.applyMethodFieldWrites(site)
}

// closeMethodFieldWrites applies every staged call's field-write summary to
// its receiver, to a fixpoint: method-calls-method composes the same way.
func (r *Result) closeMethodFieldWrites() {
	// A changed pass strictly grows some entity-keyed set, so passes past the entity count mean a bug.
	limit := len(r.Entities) + 1
	for iter, changed := 0, true; changed; iter++ {
		if iter > limit {
			panic("sem: closeMethodFieldWrites did not converge")
		}
		changed = false
		for _, site := range r.methodFieldCalls {
			if r.applyMethodFieldWrites(site) {
				changed = true
			}
		}
	}
}

func (r *Result) applyMethodFieldWrites(site methodFieldCallSite) bool {
	info := r.Fn(site.method)
	if info.Owner == 0 || info.SelfParam == 0 {
		return false
	}
	changed := false
	for _, field := range r.typeDecl(info.Owner).Fields {
		if r.applyFieldWriteSummary(site, field, info) {
			changed = true
		}
	}
	return changed
}

// applyFieldWriteSummary replays method's self.field write onto site.recv.field,
// re-resolving a parameter-sourced write against this call site's actual argument.
func (r *Result) applyFieldWriteSummary(site methodFieldCallSite, field EntityID, info *FnInfo) bool {
	srcKey := fieldFnKey{Ent: info.SelfParam, Field: field}
	dstKey := fieldFnKey{Ent: site.recv, Field: field}
	if r.fieldWitnessPoisoned[dstKey] {
		return false
	}
	if r.fieldWitnessPoisoned[srcKey] {
		return r.poisonFieldKeyGlobal(dstKey)
	}
	changed := false
	if fns := r.knownFieldFn[srcKey]; len(fns) > 0 && r.joinKnownFieldFn(dstKey, fns) {
		changed = true
	}
	srcParam, tracked := r.fieldParamEnts[srcKey]
	if !tracked {
		return changed
	}
	if w := r.localWitness[srcParam]; len(w) > 0 && r.joinLocalWitness(r.fieldParamEnt(dstKey), w) {
		changed = true
	}
	t := r.tree(site.file)
	for _, e := range info.Edges {
		if e.Param != srcParam || (e.Kind != EdgeFieldBind && e.Kind != EdgeIfaceArg) {
			continue
		}
		i := slices.Index(info.Params, e.Target)
		if i < 0 || i >= len(site.args) || t.Kind(site.args[i]) == syntax.Spread {
			continue
		}
		if r.applyFieldWriteExpr(site.file, dstKey, site.args[i], site.caller, site.node) {
			changed = true
		}
	}
	return changed
}

// applyFieldWriteExpr is recordFieldWrite's post-pass, cross-function form:
// expr is the actual argument standing in for the method's own parameter.
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
		return r.joinKnownFieldFn(key, fns)
	}
	if ent := r.chainIdentEnt(file, expr); ent != 0 && r.Entities[ent].Kind == EntParam {
		return r.addFieldBindEdge(caller, EdgeFieldBind, ent, key, file, call)
	}
	return r.poisonFieldKeyGlobal(key)
}

func (r *Result) applyWitnessFieldWriteExpr(file FileID, key fieldFnKey, expr syntax.NodeID, caller EntityID, call syntax.NodeID) bool {
	src := r.chainFieldEnt(file, expr)
	if src == 0 {
		return r.poisonFieldKeyGlobal(key)
	}
	if r.Entities[src].Kind == EntLocal {
		if w, ok := r.localWitness[src]; ok {
			return r.joinLocalWitness(r.fieldParamEnt(key), w)
		}
		return r.poisonFieldKeyGlobal(key)
	}
	return r.addFieldBindEdge(caller, EdgeIfaceArg, src, key, file, call)
}

func (r *Result) joinKnownFieldFn(key fieldFnKey, add []EntityID) bool {
	changed := false
	for _, f := range add {
		if !slices.Contains(r.knownFieldFn[key], f) {
			r.knownFieldFn[key] = append(r.knownFieldFn[key], f)
			changed = true
		}
	}
	return changed
}

// addFieldBindEdge records key's fieldBindOwners so poisonFieldKeyGlobal can
// retract a stale edge added before a later call site poisons the same key.
func (r *Result) addFieldBindEdge(fn EntityID, kind EdgeKind, target EntityID, key fieldFnKey, file FileID, node syntax.NodeID) bool {
	param := r.fieldParamEnt(key)
	info := r.Fn(fn)
	for _, e := range info.Edges {
		if e.Kind == kind && e.Target == target && e.Param == param {
			return false
		}
	}
	info.Edges = append(info.Edges, EffectEdge{Kind: kind, Target: target, Param: param, File: file, Node: node})
	if r.fieldBindOwners == nil {
		r.fieldBindOwners = map[EntityID][]EntityID{}
	}
	if !slices.Contains(r.fieldBindOwners[param], fn) {
		r.fieldBindOwners[param] = append(r.fieldBindOwners[param], fn)
	}
	return true
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
			*edges = slices.DeleteFunc(*edges, func(e EffectEdge) bool { return e.Param == ent })
		}
		delete(r.fieldBindOwners, ent)
	}
	r.fieldWitnessPoisoned[key] = true
	return true
}
