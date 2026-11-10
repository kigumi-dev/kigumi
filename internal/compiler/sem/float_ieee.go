package sem

import (
	"math"
	"math/big"

	"kigumi/internal/lang/token"
)

// ieeeFloat rounds v to the IEEE format of typ (Float unless typ is f32), subnormals included; a value past the
// largest finite one becomes ±Inf, which a constant cannot hold.
func ieeeFloat(tt *TypeTable, v *big.Float, typ TypeID) *big.Float {
	if floatPrec(tt, typ) == 24 {
		f, _ := v.Float32()
		return new(big.Float).SetPrec(24).SetFloat64(float64(f))
	}
	f, _ := v.Float64()
	return new(big.Float).SetPrec(53).SetFloat64(f)
}

// floatOp applies an arithmetic operator to two finite values of typ and rounds the result once to typ; raw is the
// result before rounding, for the diagnostic when it overflows. A zero divisor is the caller's to reject.
func floatOp(tt *TypeTable, op token.Kind, x, y *big.Float, typ TypeID) (z, raw *big.Float, ok bool) {
	raw = new(big.Float).SetPrec(256)
	switch op {
	case token.Plus:
		raw.Add(x, y)
	case token.Minus:
		raw.Sub(x, y)
	case token.Star:
		raw.Mul(x, y)
	case token.Slash:
		raw.Quo(x, y)
	case token.Percent:
		xf, _ := x.Float64()
		yf, _ := y.Float64()
		raw.SetFloat64(math.Mod(xf, yf))
	default:
		return nil, nil, false
	}
	return ieeeFloat(tt, raw, typ), raw, true
}

func floatArithOp(op token.Kind) bool {
	switch op {
	case token.Plus, token.Minus, token.Star, token.Slash, token.Percent:
		return true
	}
	return false
}

func isComparison(op token.Kind) bool {
	switch op {
	case token.EqEq, token.NotEq, token.Lt, token.LtEq, token.Gt, token.GtEq:
		return true
	}
	return false
}

func isConstNumber(v constValue) bool { return v.Kind == constInt || v.Kind == constFloat }
