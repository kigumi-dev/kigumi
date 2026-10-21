package sem

import (
	"strconv"

	"kigumi/internal/syntax"
)

func (r *Result) constFits(f FileID, n syntax.NodeID, v constValue, typ TypeID) bool {
	tt := r.Types
	switch {
	case tt.IsInteger(typ) && v.Kind == constInt:
		if v.Typ != 0 && v.Typ != typ {
			r.errAt(f, n, cTypeMismatch, typ, v.Typ)
			return false
		}
		if v.Typ == 0 && v.Shift >= tt.Width(typ) {
			r.errAt(f, n, cLiteralOutOfRange, "a shift by "+strconv.Itoa(v.Shift), typ)
			return false
		}
		if !r.intFits(v.Int, typ) {
			r.errAt(f, n, cLiteralOutOfRange, v.Int.String(), typ)
			return false
		}
	case tt.IsInteger(typ) && v.Kind == constFloat:
		r.errAt(f, n, cLiteralFloatIntoInt, v.Float.Text('g', -1), typ)
		return false
	case tt.IsFloat(typ) && v.Kind == constInt:
		if !intExactAsFloat(v.Int, tt.Width(typ)) {
			r.errAt(f, n, cLiteralOutOfRange, v.Int.String(), typ)
			return false
		}
	case tt.IsFloat(typ) && v.Kind == constFloat:
	case typ == r.untypedOf(v):
	case typ == TyPoison:
	default:
		r.errAt(f, n, cTypeMismatch, typ, r.untypedOf(v))
		return false
	}
	return true
}

// LiteralFits reports whether an integer literal is in the range of the integer type typ.
func (r *Result) LiteralFits(lit Literal, typ TypeID) bool {
	if lit.Kind != LitInt || lit.Int == nil || !r.Types.IsInteger(typ) {
		return true
	}
	return r.intFits(lit.Int, typ)
}
