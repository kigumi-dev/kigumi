package sem

import (
	"math"
	"math/big"
	"strconv"

	"kigumi/internal/compiler/syntax"
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
		if !floatFits(v.Float, tt.Width(typ)) {
			r.errAt(f, n, cLiteralOutOfRange, v.Float.Text('g', -1), typ)
			return false
		}
	case typ == r.untypedOf(v):
	case typ == TyPoison:
	default:
		r.errAt(f, n, cTypeMismatch, typ, r.untypedOf(v))
		return false
	}
	return true
}

// floatFits reports whether the literal is finite at the target width. Underflow to
// zero is allowed, like Go; overflow to infinity is not.
func floatFits(v *big.Float, bits int) bool {
	if v == nil {
		return true
	}
	if bits == 32 {
		f, _ := v.Float32()
		return !math.IsInf(float64(f), 0)
	}
	f, _ := v.Float64()
	return !math.IsInf(f, 0)
}

// LiteralFits reports whether an integer literal is in the range of the integer type typ.
func (r *Result) LiteralFits(lit Literal, typ TypeID) bool {
	if lit.Kind != LitInt || lit.Int == nil || !r.Types.IsInteger(typ) {
		return true
	}
	return r.intFits(lit.Int, typ)
}
