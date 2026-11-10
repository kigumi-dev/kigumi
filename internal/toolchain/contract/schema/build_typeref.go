package schema

import (
	"strconv"

	"kigumi/internal/compiler/sem"
)

// typeRef recurses into t's generic/const arguments and element type; nil
// for NoType.
func typeRef(res *sem.Result, t sem.TypeID) *TypeRef {
	if t == sem.NoType {
		return nil
	}
	n := res.Types.Node(t)
	display := res.TypeString(t)
	switch n.Kind {
	case sem.KPrim, sem.KUntyped:
		return &TypeRef{Kind: TRPrim, Display: display, Prim: display}
	case sem.KNamed:
		ref := &TypeRef{Kind: TRNamed, Display: display, Decl: declRefOf(res, n.Ent), WellKnown: wellKnownOf(res, n.Ent)}
		for _, a := range n.Args {
			ref.Args = append(ref.Args, typeRef(res, a))
		}
		return ref
	case sem.KIface:
		ref := &TypeRef{Kind: TRIface, Display: display, Decl: declRefOf(res, n.Ent)}
		for _, a := range n.Args {
			ref.Args = append(ref.Args, typeRef(res, a))
		}
		return ref
	case sem.KParam:
		return &TypeRef{Kind: TRParam, Display: display, Decl: declRefOf(res, n.Ent)}
	case sem.KFn, sem.KClosure:
		kind := TRFn
		if n.Kind == sem.KClosure {
			kind = TRClosure
		}
		ref := &TypeRef{Kind: kind, Display: display, Elem: typeRef(res, n.Elem), Variadic: n.Flags&sem.FnCVariadic != 0}
		for _, a := range n.Args {
			ref.Params = append(ref.Params, typeRef(res, a))
		}
		return ref
	case sem.KRef:
		return &TypeRef{Kind: TRBorrow, Display: display, Elem: typeRef(res, n.Elem), Mutable: res.Types.RefMut(t), Lifetime: typeRef(res, n.Lifetime)}
	case sem.KPtr:
		// A KPtr node's only flag is its mutability, so a nonzero flag set is
		// exactly `*mut T`; the IR keeps it because a C emitter needs `const`.
		return &TypeRef{Kind: TRPointer, Display: display, Elem: typeRef(res, n.Elem), Mutable: n.Flags != 0}
	case sem.KConst:
		value, _ := res.Types.ConstValue(t)
		return &TypeRef{Kind: TRConst, Display: display, Value: strconv.FormatInt(value, 10)}
	default:
		return &TypeRef{Kind: TRPoison, Display: display}
	}
}

func wellKnownOf(res *sem.Result, ent sem.EntityID) WellKnown {
	switch ent {
	case res.Types.OptionEnt():
		return WKOption
	case res.Types.ResultEnt():
		return WKResult
	case res.Types.ArrayEnt():
		return WKArray
	case res.Types.FutureEnt():
		return WKFuture
	case res.Types.ErrorEnt():
		return WKError
	}
	if ent != 0 && ent == res.LangItem("FixedArray") {
		return WKFixedArray
	}
	return WKNone
}
