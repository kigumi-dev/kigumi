package sem

// checkOverloadSets rejects two members with the same parameter types
// and members of one set with different visibility.
func (r *Result) checkOverloadSets() {
	for set := 1; set < len(r.Overloads); set++ {
		members := r.Overloads[set].Members
		for i := 1; i < len(members); i++ {
			m := members[i]
			e := &r.Entities[m]
			if e.Flags&EfPoison != 0 {
				continue
			}
			for _, prev := range members[:i] {
				if r.Entities[prev].Flags&EfPoison != 0 || r.Overloads[set].Owner != 0 && r.Entities[prev].Pkg != e.Pkg {
					continue
				}
				if r.sameParams(prev, m) {
					r.errAt(e.File, e.Node, cFnRedeclared, e.Name, r.paramTypesText(m))
					e.Flags |= EfPoison
					break
				}
				if r.Entities[prev].Vis != e.Vis {
					r.errAt(e.File, e.Node, cOverloadVisibility, e.Name)
					break
				}
			}
		}
	}
}

// sameParams reports two members some argument count calls alike: equal parameter types over the shortest count both accept.
func (r *Result) sameParams(a, b EntityID) bool {
	fa, fb := r.Fn(a), r.Fn(b)
	na, nb := r.Types.Node(fa.Sig), r.Types.Node(fb.Sig)
	if fa.Recv != fb.Recv || na.Flags&fnVariadic != nb.Flags&fnVariadic {
		return false
	}
	need := len(na.Args)
	if na.Flags&fnVariadic == 0 {
		need = max(r.requiredParams(fa.Params, len(na.Args)), r.requiredParams(fb.Params, len(nb.Args)))
		if need > min(len(na.Args), len(nb.Args)) {
			return false
		}
	} else if len(na.Args) != len(nb.Args) {
		return false
	}
	for i := 0; i < need; i++ {
		if na.Args[i] != nb.Args[i] {
			return false
		}
	}
	return true
}

func (r *Result) requiredParams(params []EntityID, total int) int {
	for i := 0; i < min(total, len(params)); i++ {
		if r.Entities[params[i]].Target != 0 {
			return i
		}
	}
	return total
}

func (r *Result) paramTypesText(id EntityID) string {
	out := ""
	for i, p := range r.Types.Node(r.Fn(id).Sig).Args {
		if i > 0 {
			out += ", "
		}
		out += r.TypeString(p)
	}
	return out
}
