package sem

// RegScalar is the leaf set the native backend keeps in a register: i128 and
// u128 outgrow the boxed payload and f32 rounds on construction.
func (r *Result) RegScalar(t TypeID) bool {
	switch {
	case t == TyBool, t == TyChar, t == TyF64:
		return true
	case r.Types.IsInteger(t) && t != TyI128 && t != TyU128:
		return true
	}
	return false
}

func (r *Result) DirectAgg(t TypeID) bool {
	if t == 0 {
		return false
	}
	p := &r.Types.props[t]
	switch p.agg {
	case 1:
		return true
	case -1:
		return false
	}
	// A field cycle answers "not an aggregate"; by-value recursion is E221.
	p.agg = -1
	if r.computeDirectAgg(t) {
		r.Types.props[t].agg = 1
		return true
	}
	return false
}

func (r *Result) computeDirectAgg(t TypeID) bool {
	n := r.Types.Node(t)
	if n.Kind != KNamed || r.Repr(t) != ReprDirect {
		return false
	}
	info := r.typeDecl(n.Ent)
	if info.Form != FormRecord || info.Layout != "" || len(info.Fields) == 0 {
		return false
	}
	subst := instSubst(info.Params, n.Args)
	for _, fld := range info.Fields {
		ft := r.Types.Subst(r.Entities[fld].Type, subst)
		if !r.RegScalar(ft) && !r.DirectAgg(ft) {
			return false
		}
	}
	return true
}

// The caller has already checked DirectAgg.
func (r *Result) AggFields(t TypeID) []TypeID {
	n := r.Types.Node(t)
	info := r.typeDecl(n.Ent)
	subst := instSubst(info.Params, n.Args)
	out := make([]TypeID, len(info.Fields))
	for i, fld := range info.Fields {
		out[i] = r.Types.Subst(r.Entities[fld].Type, subst)
	}
	return out
}
