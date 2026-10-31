package sem

import (
	"math/big"

	"kigumi/internal/compiler/syntax"
)

// No integer type is wider, so a larger untyped shift could only exhaust memory.
const maxUntypedShift = 4096

func (r *Result) typedConstInt(f FileID, n syntax.NodeID, z *big.Int, typ TypeID) (constValue, bool) {
	if typ != 0 && !r.intFits(z, typ) {
		r.errAt(f, n, cLiteralOutOfRange, z.String(), typ)
		return constValue{}, false
	}
	return constValue{Kind: constInt, Int: z, Typ: typ}, true
}

func (r *Result) constShiftOK(f FileID, n syntax.NodeID, x, count *big.Int, typ TypeID, left bool) bool {
	if count.Sign() < 0 {
		r.errAt(f, n, cLiteralOutOfRange, count.String(), TyUsize)
		return false
	}
	limit := int64(maxUntypedShift)
	switch {
	case typ != 0:
		limit = int64(r.Types.Width(typ))
	case !left:
		limit = int64(r.Types.Width(TyI128))
	}
	if !count.IsInt64() || count.Int64() >= limit || (left && typ == 0 && int64(x.BitLen())+count.Int64() > limit) {
		r.errAt(f, n, cLiteralOutOfRange, "a shift by "+count.String(), r.constShiftType(typ))
		return false
	}
	return true
}

func (r *Result) constShiftType(typ TypeID) TypeID {
	if typ != 0 {
		return typ
	}
	return TyI64
}
