package sem

func (r *Result) WritableInside(t TypeID) bool {
	w, _ := r.writableInside(t)
	return w
}

// WritableByParam is a WritableInside answer that rests on a type parameter: only the value at hand can tell.
func (r *Result) WritableByParam(t TypeID) bool {
	w, byParam := r.writableInside(t)
	return w && byParam
}

const (
	insideUnknown int8 = iota
	insideNone
	insideParam
	insideMut
)

// Only a whole search is memoized: a type met again on its own cycle is still being searched.
func (r *Result) writableInside(t TypeID) (bool, bool) {
	if t == 0 || int(t) >= len(r.Types.props) {
		return false, false
	}
	if m := r.Types.props[t].inside; m != insideUnknown {
		return m != insideNone, m == insideParam
	}
	s := insideSearch{r: r, seen: map[TypeID]bool{}}
	m := insideNone
	switch {
	case s.mut(t):
		m = insideMut
	case s.param:
		m = insideParam
	}
	r.Types.props[t].inside = m
	return m != insideNone, m == insideParam
}

type insideSearch struct {
	r     *Result
	seen  map[TypeID]bool
	param bool
}

// An opaque handle and a borrow field share their target by design, so the search stops there.
func (s *insideSearch) mut(t TypeID) bool {
	r, tt := s.r, s.r.Types
	if t == 0 || s.seen[t] || !r.isCopy(t) {
		return false
	}
	s.seen[t] = true
	if m := tt.props[t].inside; m != insideUnknown {
		s.param = s.param || m == insideParam
		return m == insideMut
	}
	n := tt.Node(t)
	switch {
	case n.Kind == KParam:
		s.param = true
		return false
	case n.Kind != KNamed:
		return false
	case r.isFixedArray(t):
		return true
	}
	info := r.typeDecl(n.Ent)
	subst := instSubst(info.Params, n.Args)
	switch info.Form {
	case FormDefined:
		return info.Underlying != TyPoison && s.mut(tt.Subst(info.Underlying, subst))
	case FormRecord:
		if r.RecordWritable(n.Ent) {
			return true
		}
		for _, fld := range info.Fields {
			if s.mut(tt.Subst(r.Entities[fld].Type, subst)) {
				return true
			}
		}
	case FormAdt:
		for _, v := range info.Variants {
			for _, p := range r.variant(v).Payload {
				if s.mut(tt.Subst(p, subst)) {
					return true
				}
			}
		}
	}
	return false
}

func (r *Result) RecordWritable(ent EntityID) bool {
	if ent == 0 || r.Entities[ent].Kind != EntType {
		return false
	}
	info := r.typeDecl(ent)
	if info.Form != FormRecord {
		return false
	}
	for _, fld := range info.Fields {
		if r.Entities[fld].Flags&EfMut != 0 {
			return true
		}
	}
	return false
}
