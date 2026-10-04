package sem

// fnShapeOf returns t's callable shape: t itself for a plain KFn, or the Fn/FnMut/FnOnce bound of a
// constrained type parameter; 0 when t names neither.
func (r *Result) fnShapeOf(t TypeID) TypeID {
	switch r.Types.Kind(t) {
	case KFn:
		return t
	case KParam:
		for _, con := range r.typeParam(r.Types.Node(t).Ent).Constraints {
			if con.Kind == CFn || con.Kind == CFnMut || con.Kind == CFnOnce {
				return con.Type
			}
		}
	}
	return 0
}

// paramIsEscaping decides a declared parameter's escape mode: explicit `escaping`, implied by
// async/extern(C), or (for a bodiless non-interface function) absent from nonEscapingIntrinsics.
func (r *Result) paramIsEscaping(id EntityID, info *FnInfo, typ TypeID) bool {
	shape := r.fnShapeOf(typ)
	if shape == 0 {
		return false
	}
	return r.Types.FnIsEscaping(shape) || r.impliedEscaping(id, info)
}

// impliedEscaping is paramIsEscaping's non-keyword half: true when the declaration implies every
// fn-typed parameter escapes, without a written `escaping`.
func (r *Result) impliedEscaping(id EntityID, info *FnInfo) bool {
	if info.Declared&EffAsync != 0 || info.Abi != "" {
		return true
	}
	if info.Body != 0 || (info.Owner != 0 && r.Entities[info.Owner].Kind == EntInterface) {
		return false
	}
	return !nonEscapingIntrinsics[r.intrinsicKey(id, info)]
}

// paramEscapeType bakes an implied escape mode into typ so a value/witness/alias also carries it.
// C fn pointers are excluded: checkArg doesn't unify escaping and plain C fn pointer types.
func (r *Result) paramEscapeType(id EntityID, info *FnInfo, typ TypeID) TypeID {
	if r.Types.Kind(typ) != KFn || r.Types.IsCFn(typ) || r.Types.FnIsEscaping(typ) || !r.impliedEscaping(id, info) {
		return typ
	}
	return r.Types.WithEscaping(typ)
}

// intrinsicKey names a function as "<package path>:<Owner.>Name" for nonEscapingIntrinsics, since
// names alone could collide across std packages.
func (r *Result) intrinsicKey(id EntityID, info *FnInfo) string {
	e := &r.Entities[id]
	recv := ""
	if info.Owner != 0 {
		recv = r.Entities[info.Owner].Name + "."
	}
	return r.Packages[e.Pkg].Path + ":" + recv + e.Name
}

// nonEscapingIntrinsics is the reviewed allowlist of bodiless std functions trusted non-escaping.
// A bodiless function not listed here is escaping by default.
var nonEscapingIntrinsics = map[string]bool{
	// calls f once with a borrow of the receiver's value.
	"std/alloc:Shared.with":    true,
	"std/alloc:Shared.withMut": true,
	// calls f once with a borrow over the pinned pointer.
	"std/ffi:Pin.with": true,
	// calls f once with a borrow of the element at index.
	"std/array:Array.with":    true,
	"std/array:Array.withMut": true,
}

// nonEscapingParam reports whether p is a declared parameter verified non-escaping, the only shape
// a borrow-capturing closure or forwarded callback may flow into.
func (r *Result) nonEscapingParam(p EntityID) bool {
	if p == 0 {
		return false
	}
	e := &r.Entities[p]
	return r.fnShapeOf(e.Type) != 0 && e.Flags&EfEscaping == 0
}
