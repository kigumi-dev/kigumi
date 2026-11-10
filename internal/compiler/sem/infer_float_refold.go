package sem

import (
	"math/big"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// refoldFloat recomputes a folded untyped float expression at target, the float type it adopts, so each operation rounds
// as at run time; foldBinary already ran at Float. False follows a diagnostic.
func (c *checker) refoldFloat(n syntax.NodeID, target TypeID) bool {
	if floatPrec(c.r.Types, target) == floatPrec(c.r.Types, 0) || c.info.Literals[n].Kind != constFloat || !c.foldedFloat(n) {
		return true
	}
	v, ok := c.refolded(n, target)
	if ok && v != nil {
		c.info.Literals[n] = constValue{Kind: constFloat, Float: v}
	}
	return ok
}

func (c *checker) foldedFloat(n syntax.NodeID) bool {
	k := c.t.Kind(n)
	return k == syntax.Binary || k == syntax.Unary && !isNumericLiteral(c.t, n)
}

// refolded returns n's value at typ, or nil when a leaf has no folded value; false follows a diagnostic.
func (c *checker) refolded(n syntax.NodeID, typ TypeID) (*big.Float, bool) {
	n = c.literalNode(n)
	v, ok := c.info.Literals[n]
	switch {
	case !ok || v.Kind != constInt && v.Kind != constFloat:
		return nil, true
	case v.Kind == constFloat && c.foldedFloat(n) && c.t.Kind(n) == syntax.Unary:
		x, ok := c.refolded(c.t.View(n).AsUnary().Operand().ID(), typ)
		if x == nil {
			return nil, ok
		}
		return new(big.Float).Neg(x), true
	case v.Kind == constInt || !c.foldedFloat(n):
		x := ieeeFloat(c.r.Types, toFloat(v), typ)
		if x.IsInf() {
			c.errAt(n, cLiteralOutOfRange, toFloat(v).Text('g', -1), typ)
			return nil, false
		}
		return x, true
	}
	bin := c.t.View(n).AsBinary()
	x, okX := c.refolded(bin.Left().ID(), typ)
	y, okY := c.refolded(bin.Right().ID(), typ)
	if x == nil || y == nil {
		return nil, okX && okY
	}
	if bin.Op().Kind() == token.Slash && y.Sign() == 0 {
		c.errAt(bin.Right().ID(), cConstDivZero)
		return nil, false
	}
	z, raw, ok := floatOp(c.r.Types, bin.Op().Kind(), x, y, typ)
	if ok && z.IsInf() {
		c.errAt(n, cLiteralOutOfRange, raw.Text('g', 10), typ)
		return nil, false
	}
	return z, ok
}

// foldFloatNegation keeps `-x` untyped when x is a folded untyped float, so the negation adopts the type its context
// gives it instead of defaulting to Float.
func (c *checker) foldFloatNegation(n, operand syntax.NodeID, op token.Kind, t TypeID) (TypeID, bool) {
	v, ok := c.info.Literals[c.literalNode(operand)]
	if op != token.Minus || t != TyUntypedFloat || !ok || v.Kind != constFloat {
		return 0, false
	}
	c.info.Literals[n] = constValue{Kind: constFloat, Float: new(big.Float).Neg(v.Float)}
	return TyUntypedFloat, true
}
