package sem

// countedWrapper: only Option and Result keep a counted payload inline; every other aggregate boxes around one.
func (r *Result) countedWrapper(t TypeID) bool {
	_, isOpt := r.Types.IsOption(t)
	_, _, isRes := r.Types.IsResult(t)
	return isOpt || isRes
}

// countedLeaf is a payload held inline as the one reference its boxed form already is, an opaque handle or interface whatever its type arguments.
// A resource stays out: moving it out of a scrutinee goes through the boxed drop plumbing.
func (r *Result) countedLeaf(t TypeID) bool {
	if t == TyString || t == TyBytes {
		return true
	}
	n := r.Types.Node(t)
	if n.Kind != KIface && n.Kind != KNamed {
		return false
	}
	if n.Kind == KIface || r.typeDecl(n.Ent).Form == FormOpaque && !r.isFixedArray(t) {
		return !r.Types.ContainsVar(t)
	}
	if !r.concrete(t, 0) || r.DirectAgg(t) || r.nominalScalar(t) != 0 {
		return false
	}
	return r.typeDecl(n.Ent).Form != FormResource
}

func (r *Result) concrete(t TypeID, depth int) bool {
	if t == 0 || depth > 8 {
		return false
	}
	n := r.Types.Node(t)
	switch n.Kind {
	case KParam, KVar, KPoison, KUntyped:
		return false
	}
	if n.Elem != 0 && !r.concrete(n.Elem, depth+1) {
		return false
	}
	for _, a := range n.Args {
		if !r.concrete(a, depth+1) {
			return false
		}
	}
	return true
}

// CountedAdt reports a direct Option or Result whose payload slots hold counted references: copying it retains them, dropping it releases them.
func (r *Result) CountedAdt(t TypeID) bool {
	if !r.countedWrapper(t) || !r.DirectAgg(t) {
		return false
	}
	for _, v := range r.AggVariants(t) {
		for _, pt := range r.AggPayload(t, v) {
			if r.countedLeaf(pt) || r.CountedAdt(pt) {
				return true
			}
		}
	}
	return false
}

// CountedSlot reports whether payload type pt of a CountedAdt sits in its slot as a counted reference.
func (r *Result) CountedSlot(pt TypeID) bool { return r.countedLeaf(pt) }

// A counted Option or Result nests inline only in another one, so every aggregate holding a count is one of the two.
func (r *Result) payloadInline(t, pt TypeID) bool {
	return r.directLeaf(pt) || r.countedWrapper(t) && (r.countedLeaf(pt) || r.CountedAdt(pt))
}

// CountFree reports a direct value whose bits hold no count: a copy of one shares nothing with its source.
func (r *Result) CountFree(t TypeID) bool { return r.DirectAgg(t) && !r.CountedAdt(t) }
