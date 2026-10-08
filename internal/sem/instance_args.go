package sem

func (r *Result) OwnerParams(fn EntityID) []EntityID { return r.ownerParams(fn) }

func (r *Result) IsLifetimeParam(p EntityID) bool {
	return r.Entities[p].Kind == EntTypeParam && r.typeParam(p).IsLifetime
}

// A lifetime parameter does not keep a type argument from being concrete.
func (r *Result) ConcreteArg(t TypeID) bool {
	if t == 0 || r.Types.ContainsVar(t) {
		return false
	}
	for _, p := range r.Types.Params(t) {
		if !r.IsLifetimeParam(p) {
			return false
		}
	}
	return true
}

func (tt *TypeTable) BindParams(pat, act TypeID, want map[EntityID]bool, out map[EntityID]TypeID) bool {
	if pat == act {
		return true
	}
	if pat == 0 || act == 0 {
		return false
	}
	pn, an := tt.nodes[pat], tt.nodes[act]
	if bind, ok := want[pn.Ent]; ok && pn.Kind == KParam {
		if !bind {
			return true
		}
		if prev, seen := out[pn.Ent]; seen {
			return prev == act
		}
		out[pn.Ent] = act
		return true
	}
	pf, af := pn.Flags, an.Flags
	if pn.Kind == KFn {
		pf, af = pf&^fnEscaping, af&^fnEscaping
	}
	if pn.Kind != an.Kind || pn.Ent != an.Ent || pn.Var != an.Var || pf != af || len(pn.Args) != len(an.Args) {
		return false
	}
	if (pn.Elem != 0 || an.Elem != 0) && !tt.BindParams(pn.Elem, an.Elem, want, out) {
		return false
	}
	for i := range pn.Args {
		if !tt.BindParams(pn.Args[i], an.Args[i], want, out) {
			return false
		}
	}
	return true
}
