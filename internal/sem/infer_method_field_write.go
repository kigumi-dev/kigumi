package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

type methodFieldCallSite struct {
	file     FileID
	node     syntax.NodeID
	caller   EntityID
	recv     EntityID
	method   EntityID
	req      EntityID
	callee   EntityID
	srcParam EntityID
	owner    EntityID
	args     []syntax.NodeID
}

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
	switch {
	case site.req != 0:
		changed := false
		for _, w := range r.ifaceReqImpls()[site.req] {
			info := r.Fn(w)
			if info.Owner == 0 || info.SelfParam == 0 {
				continue
			}
			for _, field := range r.typeDecl(info.Owner).Fields {
				if r.applyFieldWriteSummary(site, field, w, info.SelfParam) {
					changed = true
				}
			}
		}
		return changed
	case site.method != 0:
		info := r.Fn(site.method)
		if info.Owner == 0 || info.SelfParam == 0 {
			return false
		}
		changed := false
		for _, field := range r.typeDecl(info.Owner).Fields {
			if r.applyFieldWriteSummary(site, field, site.method, info.SelfParam) {
				changed = true
			}
		}
		return changed
	case site.callee != 0:
		if site.srcParam == 0 {
			return false
		}
		fields := r.writtenFields(site.srcParam)
		if site.owner != 0 {
			fields = r.typeDecl(site.owner).Fields
		}
		changed := false
		for _, field := range fields {
			if r.applyFieldWriteSummary(site, field, site.callee, site.srcParam) {
				changed = true
			}
		}
		return changed
	}
	return false
}

func (r *Result) applyFieldWriteSummary(site methodFieldCallSite, field, calleeFn, srcParam EntityID) bool {
	nested := r.applyNestedWriteSummary(site, field, calleeFn, srcParam)
	return r.applyFieldKeySummary(site, field, calleeFn, srcParam) || nested
}

func (r *Result) applyFieldKeySummary(site methodFieldCallSite, field, calleeFn, srcParam EntityID) bool {
	srcKey := fieldFnKey{Ent: srcParam, Field: field}
	dstKey := fieldFnKey{Ent: site.recv, Field: field}
	if r.fieldWitnessPoisoned[dstKey] {
		return false
	}
	if r.fieldWitnessPoisoned[srcKey] {
		if at, ok := r.fieldPoisonSite[srcKey]; ok {
			return r.poisonFieldKeyAt(dstKey, at.File, at.Node)
		}
		return r.poisonFieldKeyAt(dstKey, site.file, site.node)
	}
	changed := false
	if fns := r.knownFieldFn[srcKey]; len(fns) > 0 && r.joinKnownFieldFn(dstKey, fns) {
		changed = true
	}
	srcParamEnt, tracked := r.fieldParamEnts[srcKey]
	if !tracked {
		return changed
	}
	if w := r.localWitness[srcParamEnt]; len(w) > 0 && r.joinLocalWitness(r.fieldParamEnt(dstKey), w) {
		changed = true
	}
	info := r.Fn(calleeFn)
	t := r.tree(site.file)
	for _, e := range info.Edges {
		if e.Param != srcParamEnt || (e.Kind != EdgeFieldBind && e.Kind != EdgeIfaceArg) {
			continue
		}
		i := slices.Index(info.Params, e.Target)
		if i < 0 {
			if r.applyUntracedSummaryWrite(site, dstKey, e) {
				changed = true
			}
			continue
		}
		if i >= len(site.args) || t.Kind(site.args[i]) == syntax.Spread {
			continue
		}
		if r.applyFieldWriteExpr(site.file, dstKey, site.args[i], site.caller, site.node) {
			changed = true
		}
	}
	return changed
}

func (r *Result) applyUntracedSummaryWrite(site methodFieldCallSite, dstKey fieldFnKey, e EffectEdge) bool {
	switch src := r.Entities[e.Target]; {
	case e.Kind == EdgeFieldBind && (src.Kind == EntFn || src.Kind == EntClosure):
		return r.joinKnownFieldFn(dstKey, []EntityID{e.Target})
	case e.Kind == EdgeIfaceArg && r.isLiteralFieldEnt(e.Target):
		return r.joinLocalWitness(r.fieldParamEnt(dstKey), r.localWitness[e.Target])
	}
	return r.poisonFieldKeyAt(dstKey, site.file, site.node)
}
