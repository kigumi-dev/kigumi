package sem

// MayWitness is false only when no instance of the type owning cand conforms to the interface of req.
func (r *Result) MayWitness(req, cand EntityID) bool {
	iface := r.Entities[req].Parent
	owner := r.Fn(cand).Owner
	if owner == 0 || r.Entities[owner].Kind != EntType || r.Entities[iface].Kind != EntInterface || len(r.iface(iface).Params) > 0 {
		return true
	}
	switch r.typeDecl(owner).Form {
	case FormRecord, FormAdt, FormResource:
	default:
		return true
	}
	want, own, from := r.SelfType(iface), r.SelfType(owner), r.Entities[owner].Pkg
	seen := false
	for t := TypeID(0); int(t) < r.Types.Len(); t++ {
		n := r.Types.Node(t)
		if n.Kind != KNamed || n.Ent != owner || t == own && len(n.Args) > 0 {
			continue
		}
		seen = true
		if r.Types.ContainsVar(t) || len(r.Types.Params(t)) > 0 {
			return true
		}
		if _, ok := r.Implements(t, want, from); ok {
			return true
		}
	}
	return !seen
}
