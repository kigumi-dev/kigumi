package sem

import "kigumi/internal/compiler/syntax"

// boundContract: the witness picks the body that runs, so only the declared `noalloc` speaks for it.
func (r *Result) boundContract(id EntityID) (noalloc, ok bool) {
	if r.Entities[id].Kind != EntFn || r.Entities[r.Entities[id].Parent].Kind != EntInterface {
		return false, false
	}
	return r.summary(id).Noalloc, true
}

// siteFault is a witness site handing a generic callee an impl the profile rejects (impl 0 when unresolved).
type siteFault struct {
	site      pendingWitnessCallback
	impl, req EntityID
}

// firstFault returns id's first failing check in source order.
func (r *Result) firstFault(id EntityID, safe map[EntityID]bool) (*siteFault, EffectEdge, bool) {
	siteAt := func(e EffectEdge) *siteFault {
		for _, i := range r.recoverable.byOwner[id] {
			if s := r.recoverable.sites[i]; s.file == e.File && s.node == e.Node {
				if impl, req, bad := r.badWitness(s.fn, s.passes, safe); bad {
					return &siteFault{s, impl, req}
				}
			}
		}
		return nil
	}
	for _, e := range r.edgesOf(id) {
		if f := siteAt(e); f != nil {
			return f, e, true
		}
		if !r.recoverableEdgeSafe(e, safe) {
			return nil, e, true
		}
	}
	if s, impl, req, bad := r.badSiteOf(id, safe); bad {
		return &siteFault{s, impl, req}, EffectEdge{}, true
	}
	return nil, EffectEdge{}, false
}

// reportBoundCall names what a root's first unsafe path ends in, when that is a witness or an interface value.
func (r *Result) reportBoundCall(root EntityID, e EffectEdge, safe map[EntityID]bool) bool {
	if e.Target == 0 {
		return false
	}
	f, leaf := r.recoverableCause(root, e, safe)
	callee := "`" + r.Entities[e.Target].Name + "`"
	switch {
	case f != nil:
		r.reportSiteFault(f, callee, ", whose call to `"+r.Entities[f.site.fn].Name+"` runs", e.File, e.Node)
	case leaf != e && leaf.Kind == EdgeAlloc && leaf.Why == whyWitnessBind:
		what := callee + ", which binds the witnesses of `" + r.implName(leaf.Target) + "` for a call,"
		r.errAt(e.File, e.Node, cRecoverableOnlyFatalAlloc, what, "a type argument whose impl takes no witnesses (a non-generic type)")
	case r.ifaceCall(leaf):
		what := "`" + r.reqName(leaf.Target) + "` on an interface value"
		if leaf != e {
			what = callee + ", which calls " + what + ","
		}
		r.errAt(e.File, e.Node, cRecoverableOnlyFatalAlloc, what, "a requirement declared `noalloc` or a concrete receiver")
	default:
		return false
	}
	return true
}

func (r *Result) ifaceCall(e EffectEdge) bool {
	if e.Kind != EdgeCall || e.Target == 0 {
		return false
	}
	_, ok := r.boundContract(e.Target)
	return ok
}

// recoverableCause follows the first failing check down through inferred bodies.
func (r *Result) recoverableCause(root EntityID, e EffectEdge, safe map[EntityID]bool) (*siteFault, EffectEdge) {
	seen := map[EntityID]bool{root: true}
	for (e.Kind == EdgeCall || e.Kind == EdgeCallback) && e.Target != 0 && !seen[e.Target] {
		if _, fixed := r.recoverableFixed(e.Target); fixed {
			break
		}
		seen[e.Target] = true
		f, next, found := r.firstFault(e.Target, safe)
		if f != nil {
			return f, e
		}
		if b, ok := r.bindIn(e.Target, func(x EffectEdge) bool { return !r.recoverableEdgeSafe(x, safe) }); ok {
			return nil, b
		}
		if !found {
			break
		}
		e = next
	}
	return nil, e
}

// reportSiteFault: an unresolved witness already has its own error; a std impl cannot be edited,
// so its help points at the type argument.
func (r *Result) reportSiteFault(f *siteFault, callee, verb string, file FileID, node syntax.NodeID) {
	what, fix := callee, "the fallible (`try`-prefixed) form"
	if f.impl != 0 {
		name := r.implName(f.impl)
		what = callee + verb + " `" + name + "` as `" + r.reqName(f.req) + "`,"
		fix = "`noalloc` or only the fallible (`try`-prefixed) forms in `" + name + "`"
		if r.Entities[f.impl].Flags&EfStd != 0 {
			fix = "a type argument whose `" + r.Entities[f.req].Name + "` is `noalloc` or only uses the fallible (`try`-prefixed) forms"
		}
	}
	r.errAt(file, node, cRecoverableOnlyFatalAlloc, what, fix)
}

func (r *Result) implName(id EntityID) string {
	name := r.Entities[id].Name
	if owner := r.Fn(id).Owner; owner != 0 {
		name = r.Entities[owner].Name + "." + name
	}
	return name
}

// bindIn finds a witness bind in id's body that refused reports, so the message can name it over a vaguer cause.
func (r *Result) bindIn(id EntityID, refused func(EffectEdge) bool) (EffectEdge, bool) {
	for _, e := range r.edgesOf(id) {
		if e.Kind == EdgeAlloc && e.Why == whyWitnessBind && refused(e) {
			return e, true
		}
	}
	return EffectEdge{}, false
}

func (r *Result) reqName(req EntityID) string {
	return r.entityName(r.Entities[req].Parent) + "." + r.Entities[req].Name
}
