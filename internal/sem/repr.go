package sem

import "kigumi/internal/syntax"

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
// state; checkEffects drops it unless the type is written as a whole.
const whySharedValue = 4

// noteWholeSelfAssign records the receiver type of a `self = ...`, the
// only statement that writes a value through the storage it was built in.
func (c *checker) noteWholeSelfAssign(lhs syntax.NodeID) {
	ent, ok := c.identPlaceEnt(lhs)
	if !ok || c.r.Entities[ent].Flags&EfSelf == 0 {
		return
	}
	t := c.r.Entities[ent].Type
	if n := c.r.Types.Node(t); n.Kind == KRef {
		t = n.Elem
	}
	if n := c.r.Types.Node(t); n.Kind == KNamed {
		c.r.wholeAssigned[n.Ent] = true
		return
	}
	if owner := c.r.selfOwner(ent); owner != 0 {
		c.r.wholeAssigned[owner] = true
		return
	}
	// A receiver that resolved to nothing is already reported; widening to
	// every type would bury that error under unrelated ones.
	if c.r.Types.Node(t).Kind != KPoison {
		c.r.wholeAssigned[0] = true
	}
}

// selfOwner names the type whose member declares the self parameter ent,
// 0 when the receiver never resolved to one.
func (r *Result) selfOwner(ent EntityID) EntityID {
	fn := r.Entities[ent].Parent
	if fn == 0 || r.Entities[fn].Kind != EntFn {
		return 0
	}
	owner := r.Fn(fn).Owner
	if owner == 0 || r.Entities[owner].Kind != EntType {
		return 0
	}
	return owner
}

// sharedValueAllocates reports whether a value of t with no per-value state
// still needs storage: only when the program writes a t as a whole.
func (r *Result) sharedValueAllocates(t TypeID) bool {
	if r.wholeAssigned[0] {
		return true
	}
	n := r.Types.Node(t)
	return n.Kind != KNamed || r.wholeAssigned[n.Ent]
}
