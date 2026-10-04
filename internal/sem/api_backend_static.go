package sem

// Accessors for the back ends over `static`/`static mut` and foreign data
// symbols.

// StaticValueOf returns a Kigumi-owned static's compile-time initial value;
// false for an extern static, which has none.
func (r *Result) StaticValueOf(id EntityID) (Literal, bool) {
	if r.Entities[id].Kind != EntStatic {
		return Literal{}, false
	}
	info := r.Static(id)
	if info.Owner == StaticExtern {
		return Literal{}, false
	}
	return info.Value.literal()
}

// StaticEntities lists every static entity in the module, for a back end
// building global storage.
func (r *Result) StaticEntities() []EntityID {
	var out []EntityID
	for id := 1; id < len(r.Entities); id++ {
		if r.Entities[id].Kind == EntStatic && r.Entities[id].File != 0 && r.Entities[id].Flags&EfPoison == 0 {
			out = append(out, EntityID(id))
		}
	}
	return out
}

// StaticOwnerOf exposes a static's ownership classification and
// mutability to the backends.
func (r *Result) StaticOwnerOf(id EntityID) (owner StaticOwner, mut bool) {
	info := r.Static(id)
	return info.Owner, info.Mut
}

// StaticSymbolOf resolves a static's external linkage spelling: the
// `@linkName` override, or its source identifier.
func (r *Result) StaticSymbolOf(id EntityID) string {
	info := r.Static(id)
	if info.LinkName != "" {
		return info.LinkName
	}
	return r.Entities[id].Name
}

// StaticSectionOf is a static's `@section` requirement, empty for none.
func (r *Result) StaticSectionOf(id EntityID) string {
	return r.Static(id).Section
}

// FnLinkNameOf resolves a function's external linkage spelling.
func (r *Result) FnLinkNameOf(id EntityID) string {
	info := r.Fn(id)
	if info.LinkName != "" {
		return info.LinkName
	}
	return r.Entities[id].Name
}

// FnSectionOf is a function's `@section` requirement, empty for none.
func (r *Result) FnSectionOf(id EntityID) string {
	return r.Fn(id).Section
}
