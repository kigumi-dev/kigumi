package sem

// DirectCarried: a foreign ABI is fixed by C, an async one by its Future's argument words.
func (r *Result) DirectCarried(fn EntityID) bool {
	e := &r.Entities[fn]
	// A requirement is reached through a method table or a witness, which pass everything boxed.
	if fn == 0 || e.Kind != EntFn || e.Flags&EfPoison != 0 || r.Entities[e.Parent].Kind == EntInterface {
		return false
	}
	info := r.Fn(fn)
	return !info.Naked && info.Export == "" && info.Abi == "" && info.Declared&EffAsync == 0
}

// DirectParamSlot reports whether parameter p of fn may hold a value of its
// own: a `mut self` receiver is the caller's write-back cell, a ctx param a lease.
func (r *Result) DirectParamSlot(fn, p EntityID) bool {
	return p != 0 && r.DirectCarried(fn) && !r.IsMutSelf(p) && !r.ctxParam(fn, p)
}

// DirectParamType is the type parameter p of fn travels as unboxed, 0 when it crosses as a carrier.
func (r *Result) DirectParamType(fn, p EntityID) TypeID {
	if !r.DirectParamSlot(fn, p) || !r.DirectAgg(r.Entities[p].Type) {
		return 0
	}
	return r.Entities[p].Type
}

// DirectResultType is the result type fn returns unboxed, 0 otherwise.
func (r *Result) DirectResultType(fn EntityID, ret TypeID) TypeID {
	if !r.DirectCarried(fn) || !r.DirectAgg(ret) {
		return 0
	}
	return ret
}

func (r *Result) ctxParam(fn, p EntityID) bool {
	for _, ent := range r.Fn(fn).CtxParam {
		if ent == p {
			return true
		}
	}
	return false
}
