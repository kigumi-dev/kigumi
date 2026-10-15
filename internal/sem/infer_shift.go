package sem

import (
	"math/big"

	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

const maxFoldedShift = 1024

// Shifting one untyped integer constant by another folds like + and *, so the result adopts its use site's type.
func (c *checker) foldShift(n syntax.NodeID, op token.Kind, lhs, rhs syntax.NodeID, l, r TypeID) TypeID {
	if l != TyUntypedInt || r != TyUntypedInt {
		return 0
	}
	lv, lok := c.info.Literals[c.literalNode(lhs)]
	rv, rok := c.info.Literals[c.literalNode(rhs)]
	if !lok || !rok || lv.Kind != constInt || rv.Kind != constInt {
		return 0
	}
	if rv.Int.Sign() < 0 || !rv.Int.IsInt64() || rv.Int.Int64() > maxFoldedShift {
		return 0
	}
	out := new(big.Int)
	if op == token.Shl {
		out.Lsh(lv.Int, uint(rv.Int.Int64()))
	} else {
		out.Rsh(lv.Int, uint(rv.Int.Int64()))
	}
	c.info.Literals[n] = constValue{Kind: constInt, Int: out}
	return TyUntypedInt
}

// An untyped receiver takes the context's integer type, else stays usize like the count unless its magnitude needs a wider type.
func (c *checker) shiftReceiver(lhs syntax.NodeID, l, want TypeID) TypeID {
	if l != TyUntypedInt {
		return l
	}
	if c.r.Types.IsInteger(want) && want != TyUsize {
		return c.adoptOperand(lhs, want)
	}
	v, ok := c.info.Literals[c.literalNode(lhs)]
	if !ok || v.Kind != constInt || (v.Int.Sign() >= 0 && v.Int.BitLen() <= 64) {
		return l
	}
	return c.adoptOperand(lhs, magnitudeType(v.Int))
}

func magnitudeType(v *big.Int) TypeID {
	switch {
	case v.IsInt64():
		return TyI64
	case v.Sign() < 0 || v.BitLen() <= 127:
		return TyI128
	}
	return TyU128
}
