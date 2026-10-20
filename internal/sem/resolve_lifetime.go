package sem

import (
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

// `&'a T` and `&T` share one interned type; the tag lives on the
// declaration instead.
func (r *Result) resolveRefType(f FileID, scope ScopeID, n syntax.NodeID, ref syntax.TypeRefView, pos typePos) TypeID {
	var lt EntityID
	if ref.Op().Kind() == token.Lifetime {
		lt = r.resolveLifetimeName(f, scope, n, ref.Op().Text())
	}
	switch {
	case pos == posParam || pos == posLocal:
	case pos == posField && lt == 0:
		r.errAt(f, n, cLifetimeFieldAnnot)
	case pos == posReturn && lt == 0:
		r.errAt(f, n, cLifetimeReturnAnnot)
	case pos != posField && pos != posReturn:
		r.errAt(f, n, cBorrowPosition, typePosNames[pos])
	}
	elem := r.resolveType(f, scope, ref.Pointee().ID(), posTypeArg)
	return r.Types.Ref(elem, ref.Flags()&syntax.FlagMut != 0)
}

// Only declareGenerics binds such a name, so a hit is always a lifetime
// parameter.
func (r *Result) resolveLifetimeName(f FileID, scope ScopeID, n syntax.NodeID, name string) EntityID {
	if name == "'static" {
		r.errAt(f, n, cStaticLifetime)
		return 0
	}
	b, _, ok := r.lookup(scope, name)
	if (!ok || b.Ent == 0) && r.localLifetimeFn != 0 && r.Scopes[scope].Fn == r.localLifetimeFn {
		return r.localLifetime(r.localLifetimeFn, name)
	}
	if !ok || b.Ent == 0 {
		r.errAt(f, n, cLifetimeUnresolved, name)
		return 0
	}
	return b.Ent
}

// resolveType already validated and reported this; a failed lookup yields 0 rather than
// a duplicate diagnostic. cLifetimeMultiArg rejects more than one lifetime argument instead of silently keeping the first.
func (r *Result) explicitLifetimeOf(f FileID, scope ScopeID, n syntax.NodeID) EntityID {
	t := r.tree(f)
	switch t.Kind(n) {
	case syntax.TypeRef:
		ref := t.View(n).AsTypeRef()
		if ref.Op().Kind() != token.Lifetime {
			return 0
		}
		return r.lookupLifetimeName(scope, ref.Op().Text())
	case syntax.TypePath:
		args := t.View(n).AsTypePath().Args()
		if !args.Valid() {
			return 0
		}
		var lt EntityID
		for a := range args.IDs() {
			if t.Kind(a) != syntax.TypeLifetime {
				continue
			}
			if lt != 0 {
				r.errAt(f, n, cLifetimeMultiArg)
				return 0
			}
			lt = r.lookupLifetimeName(scope, t.View(a).Tok().Text())
		}
		return lt
	}
	return 0
}

// Unlike an ordinary type-param var, never reports cInferUnresolved; settles
// to fallback's KParam instead, since a return-only lifetime may never be
// touched by unification.
func (v *varStore) freshLifetime(origin syntax.NodeID, param, fallback EntityID) TypeID {
	t := v.fresh(origin, v.r.Entities[param].Name)
	i, _ := v.index(t)
	v.lifetimeDefault[i] = v.r.Types.Param(fallback)
	return t
}

// Returns 0 for none or more than one lifetime parameter.
func (r *Result) soleLifetimeParam(fn EntityID) EntityID {
	if fn == 0 || r.Entities[fn].Kind != EntFn {
		return 0
	}
	var sole EntityID
	for _, p := range r.Fn(fn).TypeParams {
		if !r.typeParam(p).IsLifetime {
			continue
		}
		if sole != 0 {
			return 0
		}
		sole = p
	}
	return sole
}

func (r *Result) lookupLifetimeName(scope ScopeID, name string) EntityID {
	b, _, ok := r.lookup(scope, name)
	if !ok {
		return 0
	}
	return b.Ent
}

// Unlike a bare reference's tag, this value becomes part of the resulting
// TypeID: two instantiations of the same generic record with different
// lifetimes are meant to stay distinct types.
func (r *Result) resolveLifetimeArg(f FileID, scope ScopeID, n syntax.NodeID) TypeID {
	t := r.tree(f)
	if t.Kind(n) != syntax.TypeLifetime {
		r.errAt(f, n, cLifetimeArgKind, "this", "a lifetime", "a type")
		return TyPoison
	}
	ent := r.resolveLifetimeName(f, scope, n, t.View(n).Tok().Text())
	if ent == 0 {
		return TyPoison
	}
	return r.Types.Param(ent)
}
