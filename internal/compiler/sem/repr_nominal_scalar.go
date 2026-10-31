package sem

// nominalScalar is the scalar a defined type like `type UserId = u64` stores, or 0.
func (r *Result) nominalScalar(t TypeID) TypeID {
	n := r.Types.Node(t)
	if n.Kind != KNamed || r.typeDecl(n.Ent).Form != FormDefined {
		return 0
	}
	if u := r.underlyingRepr(t); r.RegScalar(u) || WideScalar(u) {
		return u
	}
	return 0
}

// slotTypes is ts as a direct aggregate stores them: a nominal scalar in its scalar's slot.
func (r *Result) slotTypes(ts []TypeID) []TypeID {
	for i, t := range ts {
		if u := r.nominalScalar(t); u != 0 {
			ts[i] = u
		}
	}
	return ts
}
