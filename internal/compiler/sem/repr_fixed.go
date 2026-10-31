package sem

func (r *Result) isFixedArray(t TypeID) bool {
	tt := r.Types
	if t == 0 || tt.Kind(t) != KNamed {
		return false
	}
	ent := tt.Node(t).Ent
	return ent != 0 && ent == r.langItem(0, 0, "FixedArray")
}

// Inline cap: 4096 bytes, one page; bigger arrays stay boxed.
const maxInlineFixed = 4096

func (r *Result) directFixed(t TypeID) bool {
	elem, n, ok := r.fixedArrayShape(t)
	if !ok || n < 0 || !r.directLeaf(elem) {
		return false
	}
	return uint64(n) <= maxInlineFixed/max(r.inlineBytes(elem), 1)
}

func (r *Result) inlineBytes(t TypeID) uint64 {
	tt := r.Types
	switch {
	case t == TyUnit:
		return 0
	case t == TyBool:
		return 1
	case t == TyChar:
		return 4
	case tt.IsNumeric(t):
		return uint64(tt.Width(t)) / 8
	}
	if elem, n, ok := r.fixedArrayShape(t); ok {
		return uint64(n) * r.inlineBytes(elem)
	}
	if r.DirectAdt(t) {
		var most uint64
		for _, v := range r.AggVariants(t) {
			most = max(most, r.sumBytes(r.AggPayload(t, v)))
		}
		return 8 + most
	}
	if tt.Kind(t) == KClosure {
		return 8 + r.sumBytes(r.AggFields(t))
	}
	return r.sumBytes(r.AggFields(t))
}

func (r *Result) sumBytes(ts []TypeID) uint64 {
	var n uint64
	for _, t := range ts {
		n += r.inlineBytes(t)
	}
	return n
}

func (r *Result) FixedElem(t TypeID) (elem TypeID, n int64, ok bool) { return r.fixedArrayShape(t) }

func (r *Result) DirectFixed(t TypeID) bool {
	_, _, ok := r.fixedArrayShape(t)
	return ok && r.DirectAgg(t)
}

func (r *Result) IsFixedArray(t TypeID) bool { return r.isFixedArray(t) }
