package sem

import "slices"

func (r *Result) witnessReplacesSelf(req EntityID, iface TypeID) bool {
	if w, resolved := r.witnessReplacingSelfConforming(req, iface); resolved {
		return w != 0
	}
	return r.witnessReplacingSelf(req) != 0
}

// A generic type can conform only as an instance (Option[Int] to Taker[Int]), so queried conformances count too.
func (r *Result) queriedWitnessReplacingSelf(iface TypeID, idx int) EntityID {
	for key, res := range r.conformMemo {
		if key.iface == iface && res.ok && idx < len(res.witnesses) && r.replacesSelf(res.witnesses[idx]) {
			return res.witnesses[idx]
		}
	}
	return 0
}

// Each candidate is judged from its own package, where its private methods count as witnesses.
// The sweep's own conforms calls pollute witnessPairs with candidates no real call site selected, so they're rolled back once decided.
func (r *Result) witnessReplacingSelfConforming(req EntityID, iface TypeID) (EntityID, bool) {
	if iface == 0 || iface == TyPoison || r.Types.Kind(iface) != KIface || len(r.Types.Params(iface)) > 0 {
		return 0, false
	}
	reqs := r.iface(r.Types.Node(iface).Ent).Reqs
	idx := slices.Index(reqs, req)
	if idx < 0 {
		return 0, false
	}
	if w := r.queriedWitnessReplacingSelf(iface, idx); w != 0 {
		return w, true
	}
	pairsBefore := len(r.witnessPairs)
	defer r.rollbackWitnessPairs(pairsBefore)
	for id := EntityID(1); id < EntityID(len(r.Entities)); id++ {
		if r.Entities[id].Kind != EntType {
			continue
		}
		t := r.ownerInstance(id)
		if res := r.conforms(t, iface, r.Entities[id].Pkg); res.ok && r.replacesSelf(res.witnesses[idx]) {
			return res.witnesses[idx], true
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
