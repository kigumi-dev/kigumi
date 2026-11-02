package sem

// holdsMutField reports whether a value of t may reach a `mut` field, whose object a slice shares
// with its source instead of copying; an open type may be anything.
func (r *Result) holdsMutField(t TypeID, visited map[TypeID]bool) bool {
	if t == 0 || visited[t] {
		return false
	}
	switch r.Types.Kind(t) {
	case KParam, KVar, KIface, KClosure:
		return true
	case KNamed:
	default:
		return false
	}
	visited[t] = true
	n := r.Types.Node(t)
	decl := r.typeDecl(n.Ent)
	subst := instSubst(decl.Params, n.Args)
	var parts []TypeID
	switch decl.Form {
	case FormDefined:
		parts = []TypeID{decl.Underlying}
	case FormRecord, FormResource:
		for _, fld := range decl.Fields {
			if r.Entities[fld].Flags&EfMut != 0 {
				return true
			}
			parts = append(parts, r.field(fld).Type)
		}
	case FormAdt:
		for _, v := range decl.Variants {
			parts = append(parts, r.variant(v).Payload...)
		}
	default:
		return true
	}
	for _, p := range parts {
		if r.holdsMutField(r.Types.Subst(p, subst), visited) {
			return true
		}
	}
	return false
}
