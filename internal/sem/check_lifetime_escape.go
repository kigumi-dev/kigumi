package sem

import "kigumi/internal/syntax"

// checkFieldLifetimeEscape rejects a field type with an unbound lifetime
// parameter not declared by the owner; tag is non-zero only
// for a bare `&'a T` field, already checked by resolveRefType (E1017), so
// this only fires for a named type like `View['a]`.
func (r *Result) checkFieldLifetimeEscape(f FileID, n syntax.NodeID, owner EntityID, typ TypeID, tag EntityID) {
	if tag != 0 || !r.typeHasLifetime(typ) {
		return
	}
	own := map[EntityID]bool{}
	for _, p := range r.typeDecl(owner).Params {
		if r.typeParam(p).IsLifetime {
			own[p] = true
		}
	}
	for _, p := range r.Types.Params(typ) {
		if r.typeParam(p).IsLifetime && !own[p] {
			r.errAt(f, n, cLifetimeEscapes, r.TypeString(typ), "a field without a matching lifetime parameter")
			return
		}
	}
}

// lifetimeEscapeKey identifies one escaping (carrier, argument) pair, shared between
// checkLifetimeEscapeArgs's two call sites, which routinely resolve to the same escape.
type lifetimeEscapeKey struct {
	ent EntityID
	arg TypeID
}

// checkLifetimeEscapeArgs rejects a lifetime-carrying type argument to `Shared[T]` or `Future[T]`,
// written as a type or found in an expression's inferred type (checkLifetimeEscapeType).
func (r *Result) checkLifetimeEscapeArgs(f FileID, n syntax.NodeID, ent EntityID, args []TypeID) {
	a, ok := r.lifetimeEscapeArg(ent, args)
	if !ok {
		return
	}
	key := lifetimeEscapeKey{ent, a}
	if r.lifetimeEscapeSeen == nil {
		r.lifetimeEscapeSeen = map[lifetimeEscapeKey]bool{}
	}
	if r.lifetimeEscapeSeen[key] {
		return
	}
	r.lifetimeEscapeSeen[key] = true
	r.errAt(f, n, cLifetimeEscapes, r.TypeString(a), "`"+r.Entities[ent].Name+"`")
}

// checkLifetimeEscapeType is checkLifetimeEscapeArgs's counterpart for an inferred
// type: a generic call's return type is never a type-path node, so resolveTypePath misses it.
func (r *Result) checkLifetimeEscapeType(f FileID, n syntax.NodeID, t TypeID, seen map[TypeID]bool) {
	if t == 0 || t == TyPoison || seen[t] {
		return
	}
	seen[t] = true
	node := r.Types.Node(t)
	if node.Kind == KNamed {
		r.checkLifetimeEscapeArgs(f, n, node.Ent, node.Args)
	}
	r.checkLifetimeEscapeType(f, n, node.Elem, seen)
	for _, a := range node.Args {
		r.checkLifetimeEscapeType(f, n, a, seen)
	}
}

// lifetimeEscapeArg returns the first of args carrying a lifetime, for `Shared`
// or `Future`; matched by name, not Result.langItem, to avoid a false lang-item-missing.
func (r *Result) lifetimeEscapeArg(ent EntityID, args []TypeID) (TypeID, bool) {
	alloc, hasAlloc := r.pathIndex["std/alloc"]
	isShared := hasAlloc && r.Entities[ent].Name == "Shared" && r.Entities[ent].Pkg == alloc
	if !isShared && ent != r.Types.FutureEnt() {
		return 0, false
	}
	for _, a := range args {
		if r.typeHasLifetime(a) {
			return a, true
		}
	}
	return 0, false
}

// typeHasLifetime reports whether t, or a generic argument reachable
// through it, carries a named lifetime.
func (r *Result) typeHasLifetime(t TypeID) bool {
	for _, p := range r.Types.Params(t) {
		if r.typeParam(p).IsLifetime {
			return true
		}
	}
	return false
}
