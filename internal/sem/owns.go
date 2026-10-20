package sem

// CannotOwn reports that a value of type t holds no heap object, resource or closure (a reference only borrows), so releasing it frees nothing.
func (r *Result) CannotOwn(t TypeID) bool { return r.cannotOwn(t, 0) }

func (r *Result) cannotOwn(t TypeID, depth int) bool {
	if t == 0 || t == TyUnit || r.RegScalar(t) || WideScalar(t) {
		return true
	}
	if depth > 8 || t == TyString || t == TyBytes {
		return false
	}
	n := r.Types.Node(t)
	switch n.Kind {
	case KPrim, KPtr, KRef:
		return true
	case KNamed:
		return r.namedCannotOwn(n, depth)
	}
	return false
}

func (r *Result) namedCannotOwn(n typeNode, depth int) bool {
	if n.Ent == r.langItem(0, 0, "FixedArray") && len(n.Args) > 0 {
		return r.cannotOwn(n.Args[0], depth+1)
	}
	info := r.typeDecl(n.Ent)
	subst := instSubst(info.Params, n.Args)
	switch info.Form {
	case FormDefined:
		return r.cannotOwn(r.Types.Subst(info.Underlying, subst), depth+1)
	case FormRecord, FormAdt:
	default:
		return false
	}
	if info.Drop != 0 {
		return false
	}
	for _, t := range r.fieldTypes(info, subst) {
		if !r.cannotOwn(t, depth+1) {
			return false
		}
	}
	for _, v := range info.Variants {
		for _, t := range r.payloadTypes(v, subst) {
			if !r.cannotOwn(t, depth+1) {
				return false
			}
		}
	}
	return true
}
