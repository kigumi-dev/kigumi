package sem

import (
	"math/big"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

func (r *Result) typedConstInt(f FileID, n syntax.NodeID, z *big.Int, typ TypeID) (constValue, bool) {
	if constTooWide(z) {
		r.errAt(f, n, cConstTooLarge, maxConstBits)
		return constValue{}, false
	}
	if typ != 0 && !r.intFits(z, typ) {
		r.errAt(f, n, cLiteralOutOfRange, constIntText(z), typ)
		return constValue{}, false
	}
	return constValue{Kind: constInt, Int: z, Typ: typ}, true
}

func (r *Result) constShift(f FileID, n syntax.NodeID, op token.Kind, a, b constValue) (constValue, bool) {
	count := r.tree(f).View(n).AsBinary().Right().ID()
	switch {
	case a.Kind != constInt:
		r.errAt(f, n, cOperatorUndefined, op.String(), r.TypeString(r.constType(a))+", "+r.TypeString(r.constType(b)))
		return constValue{}, false
	case b.Kind == constFloat && b.Typ == 0:
		r.errAt(f, count, cLiteralFloatIntoInt, b.Float.Text('g', -1), TyUsize)
		return constValue{}, false
	case b.Kind != constInt || b.Typ != 0 && b.Typ != TyUsize:
		r.errAt(f, count, cShiftCountUsize, r.constType(b))
		return constValue{}, false
	}
	width := 0
	if a.Typ != 0 {
		width = r.Types.Width(a.Typ)
	}
	z, fault := r.exactShift(a.Int, b.Int, width, op == token.Shl)
	switch fault {
	case shiftCountRange:
		r.errAt(f, count, cLiteralOutOfRange, constIntText(b.Int), TyUsize)
	case shiftPastWidth:
		r.errAt(f, count, cConstShiftRange, constIntText(b.Int), a.Typ, width)
	case shiftTooLarge:
		r.errAt(f, n, cConstTooLarge, maxConstBits)
	default:
		return r.typedConstInt(f, n, z, a.Typ)
	}
	return constValue{}, false
}

func (r *Result) constType(v constValue) TypeID {
	if v.Typ != 0 {
		return v.Typ
	}
	return r.untypedOf(v)
}

// declaredConst gives a const's value its declared type, so an integer initializer of a float const is a float.
func (r *Result) declaredConst(v constValue, typ TypeID) constValue {
	switch {
	case v.Kind == constInt && r.Types.IsInteger(typ):
		v.Typ = typ
	case v.Kind == constInt && r.Types.IsFloat(typ):
		v = constValue{Kind: constFloat, Typ: typ, Float: new(big.Float).SetPrec(floatPrec(r.Types, typ)).SetInt(v.Int)}
	case v.Kind == constFloat && r.Types.IsFloat(typ):
		v.Float = new(big.Float).SetPrec(floatPrec(r.Types, typ)).Set(v.Float)
		v.Typ = typ
	}
	return v
}
