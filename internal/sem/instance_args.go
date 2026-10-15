package sem

import "slices"

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

// EraseLifetimes renames the lifetime parameters in args to shared stand-ins in order of appearance,
// so instantiations that differ only in lifetime names are one.
func (r *Result) EraseLifetimes(args []TypeID) []TypeID {
	var seen []EntityID
	for _, a := range args {
		for _, p := range r.Types.Params(a) {
			if r.IsLifetimeParam(p) && !slices.Contains(seen, p) {
				seen = append(seen, p)
			}
		}
	}
	if len(seen) == 0 {
		return args
	}
	sub := map[EntityID]TypeID{}
	for k, p := range seen {
		sub[p] = r.Types.Param(r.standInLifetime(k, p))
	}
	out := make([]TypeID, len(args))
	for i, a := range args {
		out[i] = r.Types.Subst(a, sub)
	}
	return out
}

func (r *Result) standInLifetime(k int, like EntityID) EntityID {
	for len(r.lifetimeStandIns) <= k {
		e := r.Entities[like]
		e.Detail = r.addTypeParam(TypeParamInfo{IsLifetime: true})
		e.Name = "'_"
		r.lifetimeStandIns = append(r.lifetimeStandIns, r.newEntity(e))
	}
	return r.lifetimeStandIns[k]
}
