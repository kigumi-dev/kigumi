package sem

import (
	"math/big"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// Shifting one untyped integer constant by another folds like + and *, so the result adopts its use site's type;
// this covers operands the untyped constant evaluator does not reach, such as package-qualified constants.
func (c *checker) foldShift(n syntax.NodeID, op token.Kind, lhs, rhs syntax.NodeID, l, r TypeID) TypeID {
	if l != TyUntypedInt || r != TyUntypedInt {
		return 0
	}
	lv, lok := c.info.Literals[c.literalNode(lhs)]
	rv, rok := c.info.Literals[c.literalNode(rhs)]
	if !lok || !rok || lv.Kind != constInt || rv.Kind != constInt {
		return 0
	}
	out, fault := c.r.exactShift(lv.Int, rv.Int, 0, op == token.Shl)
	switch fault {
	case shiftCountRange:
		// Unfolded, the count takes `usize` and is refused at its own span like any run-time count.
		return 0
	case shiftTooLarge:
		c.errAt(n, cConstTooLarge, maxConstBits)
		return TyPoison
	}
	c.info.Literals[n] = constValue{Kind: constInt, Int: out}
	return TyUntypedInt
}

// checkConstCount refuses a shift whose count is a known constant at or past the receiver's width, which can only panic.
func (c *checker) checkConstCount(rhs syntax.NodeID, receiver TypeID) bool {
	v, ok := c.info.Literals[c.literalNode(rhs)]
	bits := c.r.Types.Width(receiver)
	if !ok || v.Kind != constInt || bits == 0 || v.Int.Cmp(big.NewInt(int64(bits))) < 0 {
		return true
	}
	c.errAt(rhs, cConstShiftRange, constIntText(v.Int), receiver, bits)
	return false
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
