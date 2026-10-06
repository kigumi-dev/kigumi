package sem

import "slices"

func (r *Result) witnessReplacesSelf(req EntityID, iface TypeID, pkg PackageID) bool {
	if w, resolved := r.witnessReplacingSelfConforming(req, iface, pkg); resolved {
		return w != 0
	}
	return r.witnessReplacingSelf(req) != 0
}

// A witness private to another package looks unresolved from pkg, so it falls back to the owner-scoped approximation instead of being cleared.
// The sweep's own conforms calls pollute witnessPairs with candidates no real call site selected, so they're rolled back once decided.
func (r *Result) witnessReplacingSelfConforming(req EntityID, iface TypeID, pkg PackageID) (EntityID, bool) {
	if iface == 0 || iface == TyPoison || r.Types.Kind(iface) != KIface {
		return 0, false
	}
	reqs := r.iface(r.Types.Node(iface).Ent).Reqs
	idx := slices.Index(reqs, req)
	if idx < 0 {
		return 0, false
	}
	pairsBefore := len(r.witnessPairs)
	defer r.rollbackWitnessPairs(pairsBefore)
	for id := EntityID(1); id < EntityID(len(r.Entities)); id++ {
		if r.Entities[id].Kind != EntType {
			continue
		}
		t := r.ownerInstance(id)
		res := r.conforms(t, iface, pkg)
		if res.ok {
			if r.replacesSelf(res.witnesses[idx]) {
				return res.witnesses[idx], true
			}
			continue
		}
		if w := r.mutSelfReplacerOwnedBy(id, req); w != 0 && !r.visibleFrom(r.Entities[w].Vis, pkg) {
			return w, true
		}
	}
	return 0, true
}

func (r *Result) rollbackWitnessPairs(n int) {
	for _, p := range r.witnessPairs[n:] {
		delete(r.witnessSeen, p)
	}
	r.witnessPairs = r.witnessPairs[:n]
}

// The conservative fallback for a candidate conforms could not clear.
func (r *Result) mutSelfReplacerOwnedBy(owner, req EntityID) EntityID {
	name, arity := r.Entities[req].Name, len(r.Fn(req).Params)
	for id := EntityID(1); id < EntityID(len(r.Entities)); id++ {
		e := &r.Entities[id]
		if id == req || e.Kind != EntFn || e.Name != name {
			continue
		}
		fi := r.Fn(id)
		if fi.Recv != RecvMut || len(fi.Params) != arity || fi.Owner != owner {
			continue
		}
		if r.replacesSelf(id) {
			return id
		}
	}
	return 0
}
