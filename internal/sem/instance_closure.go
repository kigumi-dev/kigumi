package sem

// InstantiateClosure mints a copy of cl typed under subst, which may also rename the closures it captures.
func (r *Result) InstantiateClosure(cl EntityID, subst map[EntityID]TypeID) EntityID {
	info := *r.closure(cl)
	info.Sig = r.Types.Subst(info.Sig, subst)
	info.Ret = r.Types.Subst(info.Ret, subst)
	info.Captures = make([]Capture, len(info.Captures))
	for i, c := range r.closure(cl).Captures {
		le := r.Entities[c.Local]
		le.Type = r.Types.Subst(le.Type, subst)
		info.Captures[i] = Capture{Local: r.newEntity(le), Mode: c.Mode, OwnCell: c.OwnCell}
	}
	e := r.Entities[cl]
	e.Detail = r.addClosure(info)
	id := r.newEntity(e)
	if r.closureOrigins == nil {
		r.closureOrigins = map[EntityID]EntityID{}
	}
	r.closureOrigins[id] = cl
	return id
}

func (r *Result) ClosureOrigin(cl EntityID) EntityID {
	if gen, ok := r.closureOrigins[cl]; ok {
		return gen
	}
	return cl
}
