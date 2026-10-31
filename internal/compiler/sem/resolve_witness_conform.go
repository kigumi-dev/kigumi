package sem

import "slices"

func (r *Result) witnessReplacesSelf(req EntityID, iface TypeID) bool {
	if w, resolved := r.witnessReplacingSelfConforming(req, iface); resolved {
		return w != 0
	}
	return r.witnessReplacingSelf(req) != 0
}

func (r *Result) queriedWitnessReplacingSelf(iface TypeID, idx int) EntityID {
	for key, res := range r.conformMemo {
		if key.iface == iface && res.ok && idx < len(res.witnesses) && r.replacesSelf(res.witnesses[idx]) {
			return res.witnesses[idx]
		}
	}
	return 0
}

// Only conformances some instantiation asked for count; a type nothing instantiates the interface with never reaches this call.
func (r *Result) witnessReplacingSelfConforming(req EntityID, iface TypeID) (EntityID, bool) {
	if iface == 0 || iface == TyPoison || r.Types.Kind(iface) != KIface || len(r.Types.Params(iface)) > 0 {
		return 0, false
	}
	idx := slices.Index(r.iface(r.Types.Node(iface).Ent).Reqs, req)
	if idx < 0 {
		return 0, false
	}
	return r.queriedWitnessReplacingSelf(iface, idx), true
}
