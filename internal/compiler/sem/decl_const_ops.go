package sem

import (
	"math/big"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

func (r *Result) constUnary(f FileID, n syntax.NodeID, op token.Kind, v constValue) (constValue, bool) {
	switch {
	case op == token.Minus && v.Kind == constInt:
		res, ok := r.typedConstInt(f, n, new(big.Int).Neg(v.Int), v.Typ)
		res.Shift = v.Shift
		return res, ok
	case op == token.Minus && v.Kind == constFloat:
		return constValue{Kind: constFloat, Typ: v.Typ, Float: new(big.Float).SetPrec(floatPrec(r.Types, v.Typ)).Neg(v.Float)}, true
	case op == token.Bang && v.Kind == constBool:
		return constValue{Kind: constBool, Bool: !v.Bool}, true
	case op == token.Tilde && v.Kind == constInt:
		z := new(big.Int).Not(v.Int)
		if v.Typ != 0 && !r.Types.IsSigned(v.Typ) {
			z.Add(z, new(big.Int).Lsh(big.NewInt(1), uint(r.Types.Width(v.Typ))))
		}
		return r.typedConstInt(f, n, z, v.Typ)
	}
	r.errAt(f, n, cConstNotComptime, "this operator")
	return constValue{}, false
}

func (r *Result) constBinary(f FileID, n syntax.NodeID, op token.Kind, a, b constValue) (constValue, bool) {
	fail := func() (constValue, bool) {
		r.errAt(f, n, cConstNotComptime, "this operator")
		return constValue{}, false
	}
	if a.Kind == constInt && b.Kind == constInt {
		x, y := a.Int, b.Int
		if op != token.Shl && op != token.Shr && constIntTooLarge(op, x, y) {
			r.errAt(f, n, cConstTooLarge, maxConstBits)
			return constValue{}, false
		}
		z := new(big.Int)
		typ := a.Typ
		if typ == 0 {
			typ = b.Typ
		} else if b.Typ != 0 && b.Typ != typ && op != token.Shl && op != token.Shr {
			r.errAt(f, n, cTypeMismatch, typ, b.Typ)
			return constValue{}, false
		}
		shift := max(a.Shift, b.Shift)
		if op == token.Shl || op == token.Shr {
			typ = a.Typ
			if !r.constShiftOK(f, n, x, y, typ, op == token.Shl) {
				return constValue{}, false
			}
			if typ == 0 {
				shift = max(shift, int(y.Int64()))
			}
		}
		switch op {
		case token.Plus:
			z.Add(x, y)
		case token.Minus:
			z.Sub(x, y)
		case token.Star:
			z.Mul(x, y)
		case token.Slash, token.Percent:
			if y.Sign() == 0 {
				r.errAt(f, n, cConstDivZero)
				return constValue{}, false
			}
			if op == token.Slash {
				z.Quo(x, y)
			} else {
				z.Rem(x, y)
			}
		case token.Amp:
			z.And(x, y)
		case token.Pipe:
			z.Or(x, y)
		case token.Caret:
			z.Xor(x, y)
		case token.Shl:
			z.Lsh(x, uint(y.Uint64()))
		case token.Shr:
			z.Rsh(x, uint(y.Uint64()))
		case token.EqEq, token.NotEq, token.Lt, token.LtEq, token.Gt, token.GtEq:
			return constValue{Kind: constBool, Bool: compareResult(op, x.Cmp(y))}, true
		default:
			return fail()
		}
		res, ok := r.typedConstInt(f, n, z, typ)
		res.Shift = shift
		return res, ok
	}
	if (a.Kind == constInt || a.Kind == constFloat) && (b.Kind == constInt || b.Kind == constFloat) {
		typ := a.Typ
		if typ == 0 {
			typ = b.Typ
		} else if b.Typ != 0 && b.Typ != typ {
			r.errAt(f, n, cTypeMismatch, typ, b.Typ)
			return constValue{}, false
		}
		// Float arithmetic follows IEEE at the declared type, exactly as the VM and native code do.
		prec := floatPrec(r.Types, typ)
		x, y := roundFloat(toFloat(a), prec), roundFloat(toFloat(b), prec)
		z := new(big.Float).SetPrec(prec)
		switch op {
		case token.Plus:
			z.Add(x, y)
		case token.Minus:
			z.Sub(x, y)
		case token.Star:
			z.Mul(x, y)
		case token.Slash:
			if y.Sign() == 0 {
				r.errAt(f, n, cConstDivZero)
				return constValue{}, false
			}
			z.Quo(x, y)
		case token.EqEq, token.NotEq, token.Lt, token.LtEq, token.Gt, token.GtEq:
			return constValue{Kind: constBool, Bool: compareResult(op, x.Cmp(y))}, true
		default:
			return fail()
		}
		return constValue{Kind: constFloat, Typ: typ, Float: z}, true
	}
	if a.Kind == constBool && b.Kind == constBool {
		switch op {
		case token.AndAnd:
			return constValue{Kind: constBool, Bool: a.Bool && b.Bool}, true
		case token.OrOr:
			return constValue{Kind: constBool, Bool: a.Bool || b.Bool}, true
		case token.EqEq:
			return constValue{Kind: constBool, Bool: a.Bool == b.Bool}, true
		case token.NotEq:
			return constValue{Kind: constBool, Bool: a.Bool != b.Bool}, true
		}
	}
	if a.Kind == constString && b.Kind == constString && (op == token.EqEq || op == token.NotEq) {
		return constValue{Kind: constBool, Bool: (a.Str == b.Str) == (op == token.EqEq)}, true
	}
	return fail()
}

// floatPrec is the mantissa width the declared float type keeps; an untyped expression defaults to Float.
func floatPrec(tt *TypeTable, typ TypeID) uint {
	if typ != 0 && tt.IsFloat(typ) && tt.Width(typ) == 32 {
		return 24
	}
	return 53
}

func roundFloat(v *big.Float, prec uint) *big.Float {
	return new(big.Float).SetPrec(prec).Set(v)
}

func toFloat(v constValue) *big.Float {
	if v.Kind == constFloat {
		return v.Float
	}
	return new(big.Float).SetPrec(256).SetInt(v.Int)
}

func compareResult(op token.Kind, c int) bool {
	switch op {
	case token.EqEq:
		return c == 0
	case token.NotEq:
		return c != 0
	case token.Lt:
		return c < 0
	case token.LtEq:
		return c <= 0
	case token.Gt:
		return c > 0
	}
	return c >= 0
}

const maxConstBits = 1 << 16

// constIntTooLarge reports an operation whose result would pass maxConstBits, before it allocates.
func constIntTooLarge(op token.Kind, x, y *big.Int) bool {
	switch op {
	case token.Shl:
		return y.Sign() < 0 || !y.IsUint64() || y.Uint64() > maxConstBits
	case token.Shr:
		return y.Sign() < 0 || !y.IsUint64()
	case token.Star:
		return x.BitLen()+y.BitLen() > maxConstBits
	case token.Plus, token.Minus:
		return max(x.BitLen(), y.BitLen()) >= maxConstBits
	}
	return false
}
