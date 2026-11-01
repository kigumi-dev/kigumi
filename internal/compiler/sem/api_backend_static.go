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
	if info.Thunk != 0 {
		v, ok := r.Files[r.Entities[id].File].Literals[info.Init]
		if !ok {
			return Literal{}, false
		}
		return v.literal()
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

// DirectStatic reports a Kigumi-owned `static mut` of a register scalar type, which the back ends keep as a raw slot, so a store to it takes no box.
func (r *Result) DirectStatic(id EntityID) bool {
	if r.Entities[id].Kind != EntStatic {
		return false
	}
	info := r.Static(id)
	return info.Mut && info.Owner == StaticOwned && r.RegScalar(r.Entities[id].Type)
}

// ImmortalStatic reports a Kigumi-owned non-mut static whose value is a record of literals with no `mut` field at any
// depth; nothing writes it in place, so the back ends keep it as one immortal that a copy shares.
func (r *Result) ImmortalStatic(id EntityID) bool {
	if r.Entities[id].Kind != EntStatic {
		return false
	}
	if info := r.Static(id); info.Mut || info.Owner != StaticOwned {
		return false
	}
	lit, ok := r.StaticValueOf(id)
	return ok && r.fixedRecord(lit, r.Entities[id].Type)
}

func (r *Result) fixedRecord(lit Literal, typ TypeID) bool {
	ent := r.Types.Node(typ).Ent
	if lit.Kind != LitRecord || len(lit.Fields) == 0 || r.RecordWritable(ent) {
		return false
	}
	for i, f := range r.typeDecl(ent).Fields {
		switch fl := lit.Fields[i]; fl.Kind {
		case LitRecord:
			if !r.fixedRecord(fl, r.Entities[f].Type) {
				return false
			}
		case LitInt, LitFloat, LitString, LitBool, LitChar:
		default:
			return false
		}
	}
	return true
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
