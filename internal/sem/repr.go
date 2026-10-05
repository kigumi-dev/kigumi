package sem

// ReprKind is a type's own storage class: Direct never allocates for
// construction, move or drop; Carrier owns its own indirection, even nested inside a Direct aggregate's fields.
type ReprKind uint8

const (
	ReprDirect ReprKind = iota
	ReprCarrier
)

// Repr classifies t; memoized in props alongside isCopy.
func (r *Result) Repr(t TypeID) ReprKind {
	if t == 0 {
		return ReprDirect
	}
	p := &r.Types.props[t]
	switch p.repr {
	case 1:
		return ReprDirect
	case -1:
		return ReprCarrier
	}
	res := r.computeRepr(t)
	if res == ReprDirect {
		r.Types.props[t].repr = 1
	} else {
		r.Types.props[t].repr = -1
	}
	return res
}

func (r *Result) computeRepr(t TypeID) ReprKind {
	// String/Bytes are KPrim ids alongside the scalars, but their own
	// storage is still a dynamic buffer, so they stay out of that bucket.
	if t == TyString || t == TyBytes {
		return ReprCarrier
	}
	tt := r.Types
	n := tt.Node(t)
	switch n.Kind {
	case KPoison, KPrim, KUntyped, KFn, KPtr, KRef, KConst:
		return ReprDirect
	case KClosure:
		if n.Ent == 0 {
			return ReprCarrier
		}
		return ReprDirect
	case KIface, KVar, KParam:
		// KIface is an explicit erasure carrier; KParam isn't concrete until a generic instance substitutes it.
		return ReprCarrier
	case KNamed:
		return r.namedRepr(n)
	}
	return ReprCarrier
}

func (r *Result) namedRepr(n typeNode) ReprKind {
	info := r.typeDecl(n.Ent)
	switch info.Form {
	case FormDefined:
		if info.Underlying == 0 || info.Underlying == TyPoison {
			return ReprCarrier
		}
		return r.Repr(r.Types.Subst(info.Underlying, instSubst(info.Params, n.Args)))
	case FormOpaque:
		return ReprCarrier
	default: // FormRecord, FormResource, FormAdt: an aggregate's own fields/payloads, not an opaque runtime handle.
		return ReprDirect
	}
}

// whySharedValue marks an EdgeAlloc for building a value with no per-value
// state; checkEffects drops it, since no write reaches it through its storage.
const whySharedValue = 4
