package sem

import "kigumi/internal/syntax"

// checkAbiTypes keeps C signatures to what the C ABI can carry.
func (r *Result) checkAbiTypes(f FileID, s fnSlots, params []TypeID, ret TypeID) {
	t := r.tree(f)
	i := 0
	for p := range t.List(s.Params).IDs() {
		ps := param(t, p)
		if ps.Flags&syntax.FlagSelf != 0 || ps.Type == 0 {
			continue
		}
		if i < len(params) && !r.abiSafe(params[i]) {
			r.errAt(f, ps.Type, r.abiCode(params[i], cAbiType), r.TypeString(params[i]))
		}
		i++
	}
	if s.Ret != 0 && ret != TyUnit && ret != TyNever && !r.abiSafe(ret) {
		r.errAt(f, s.Ret, r.abiCode(ret, cAbiType), r.TypeString(ret))
	}
}

func (r *Result) abiSafe(t TypeID) bool {
	tt := r.Types
	if t == TyPoison || r.cNumeric(t) || tt.Kind(t) == KPtr || tt.IsCFn(t) || r.cRecord(t) {
		return true
	}
	if n := tt.Node(t); n.Kind == KNamed && r.typeDecl(n.Ent).Form == FormDefined {
		return r.abiSafe(r.underlyingRepr(t))
	}
	return false
}

// cNumeric is a number C can carry; i128 and u128 have no C type to match.
func (r *Result) cNumeric(t TypeID) bool { return r.Types.IsNumeric(t) && !r.wideInt(t) }

// wideInt reports whether t is i128 or u128, or is defined over one.
func (r *Result) wideInt(t TypeID) bool {
	u := r.underlyingRepr(t)
	return u == TyI128 || u == TyU128
}

// abiCode picks the diagnostic for a type a C position refuses: the 128-bit integers get their own.
func (r *Result) abiCode(t TypeID, c Code) Code {
	if r.wideInt(t) {
		return cInt128Unsupported
	}
	return c
}

// cRecord reports whether t is a user record with a C layout, which the C
// ABI moves by value.
func (r *Result) cRecord(t TypeID) bool {
	n := r.Types.Node(t)
	return n.Kind == KNamed && r.Entities[n.Ent].File != 0 && r.typeDecl(n.Ent).Layout != ""
}
