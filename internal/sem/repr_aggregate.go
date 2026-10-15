package sem

// RegScalar is the leaf set the native backend keeps in a register; i128 and
// u128 outgrow the boxed payload.
func (r *Result) RegScalar(t TypeID) bool {
	switch {
	case t == TyBool, t == TyChar, t == TyF64, t == TyF32:
		return true
	case r.Types.IsInteger(t) && t != TyI128 && t != TyU128:
		return true
	}
	return false
}

// WideScalar is a 128-bit integer: a direct aggregate stores it in a 16-byte slot, though no register holds it as a boxed payload.
func WideScalar(t TypeID) bool { return t == TyI128 || t == TyU128 }

func (r *Result) directLeaf(t TypeID) bool {
	return t == TyUnit || r.RegScalar(t) || WideScalar(t) || r.DirectAgg(t)
}

func (r *Result) DirectAgg(t TypeID) bool {
	if t == 0 {
		return false
	}
	if r.Types.Kind(t) == KClosure {
		return r.directClosure(t)
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
	res := r.computeDirectAgg(t)
	switch {
	case r.containsClosure(t):
		// A closure's captures can still change, so neither answer is kept.
		r.Types.props[t].agg = 0
	case res:
		r.Types.props[t].agg = 1
	}
	return res
}

func (r *Result) computeDirectAgg(t TypeID) bool {
	n := r.Types.Node(t)
	if n.Kind != KNamed || r.Repr(t) != ReprDirect {
		return false
	}
	if _, _, ok := r.fixedArrayShape(t); ok {
		return r.directFixed(t)
	}
	info := r.typeDecl(n.Ent)
	if info.Layout != "" {
		return false
	}
	subst := instSubst(info.Params, n.Args)
	switch info.Form {
	case FormRecord:
		return len(info.Fields) > 0 && r.leavesDirect(r.fieldTypes(info, subst))
	case FormAdt:
		return len(info.Variants) > 0 && r.variantsDirect(info, subst)
	}
	return false
}

func (r *Result) leavesDirect(ts []TypeID) bool {
	for _, t := range ts {
		if !r.directLeaf(t) {
			return false
		}
	}
	return true
}

func (r *Result) variantsDirect(info *TypeDeclInfo, subst map[EntityID]TypeID) bool {
	for _, v := range info.Variants {
		if !r.leavesDirect(r.payloadTypes(v, subst)) {
			return false
		}
	}
	return true
}

func (r *Result) fieldTypes(info *TypeDeclInfo, subst map[EntityID]TypeID) []TypeID {
	out := make([]TypeID, len(info.Fields))
	for i, fld := range info.Fields {
		out[i] = r.Types.Subst(r.Entities[fld].Type, subst)
	}
	return out
}

func (r *Result) payloadTypes(v EntityID, subst map[EntityID]TypeID) []TypeID {
	pl := r.variant(v).Payload
	out := make([]TypeID, len(pl))
	for i, p := range pl {
		out[i] = r.Types.Subst(p, subst)
	}
	return out
}

func (r *Result) AggFields(t TypeID) []TypeID {
	n := r.Types.Node(t)
	if n.Kind == KClosure {
		return r.closureEnv(n.Ent)
	}
	info := r.typeDecl(n.Ent)
	return r.fieldTypes(info, instSubst(info.Params, n.Args))
}
