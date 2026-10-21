package sem

// mayHoldResource reports whether an owned t could hold a resource by value; a generic, interface or closure slot is assumed to.
func (r *Result) mayHoldResource(t TypeID, visited map[TypeID]bool) bool {
	if t == 0 || visited[t] {
		return false
	}
	switch r.Types.Kind(t) {
	case KParam, KVar, KIface, KClosure:
		return true
	case KNamed:
		visited[t] = true
		n := r.Types.Node(t)
		decl := r.typeDecl(n.Ent)
		subst := instSubst(decl.Params, n.Args)
		switch decl.Form {
		case FormResource:
			return true
		case FormDefined:
			return r.mayHoldResource(r.Types.Subst(decl.Underlying, subst), visited)
		case FormRecord:
			for _, fld := range decl.Fields {
				if r.mayHoldResource(r.Types.Subst(r.field(fld).Type, subst), visited) {
					return true
				}
			}
			return false
		case FormAdt:
			for _, v := range decl.Variants {
				for _, pt := range r.variant(v).Payload {
					if r.mayHoldResource(r.Types.Subst(pt, subst), visited) {
						return true
					}
				}
			}
			return false
		default:
			for _, a := range n.Args {
				if r.mayHoldResource(a, visited) {
					return true
				}
			}
			return false
		}
	}
	return false
}

// HoldsResource reports whether an owned t could hold a resource, which cannot be cloned.
func (r *Result) HoldsResource(t TypeID) bool { return r.mayHoldResource(t, map[TypeID]bool{}) }
