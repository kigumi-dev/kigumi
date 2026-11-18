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
		r.closureOrigins, r.closureSubsts = map[EntityID]EntityID{}, map[EntityID]map[EntityID]TypeID{}
	}
	r.closureOrigins[id] = cl
	r.closureSubsts[id] = subst
	return id
}

// ClosureSubst is the substitution cl was minted under by InstantiateClosure, nil for a closure written in source.
func (r *Result) ClosureSubst(cl EntityID) map[EntityID]TypeID { return r.closureSubsts[cl] }

func (r *Result) ClosureOrigin(cl EntityID) EntityID {
	if gen, ok := r.closureOrigins[cl]; ok {
		return gen
	}
	return cl
}

// InstantiateClosureCapturing is cl with one more capture after its own per type in extra, each a fresh local
// named name.
func (r *Result) InstantiateClosureCapturing(cl EntityID, extra []TypeID, name string) (EntityID, []EntityID) {
	id := r.InstantiateClosure(cl, nil)
	var locals []EntityID
	for _, t := range extra {
		e := r.Entities[cl]
		l := r.newEntity(Entity{Kind: EntLocal, Name: name, Pkg: e.Pkg, File: e.File, Node: e.Node, Parent: id, Type: t})
		r.Entities[l].Detail = r.addLocal(LocalInfo{})
		locals = append(locals, l)
	}
	info := r.closure(id)
	for _, l := range locals {
		info.Captures = append(info.Captures, Capture{Local: l, Mode: CapCopy})
	}
	return id, locals
}
