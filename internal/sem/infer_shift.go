package sem

import (
	"cmp"
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

func shiftOp(op token.Kind) bool { return op == token.Shl || op == token.Shr }

func shiftOperandOp(op token.Kind) bool {
	switch op {
	case token.Plus, token.Minus, token.Star, token.Slash, token.Percent, token.Amp, token.Pipe, token.Caret,
		token.EqEq, token.NotEq, token.Lt, token.LtEq, token.Gt, token.GtEq:
		return true
	}
	return shiftOp(op)
}

// shiftShaped holds for a shift, or arithmetic or a negation whose operand is one, so its untyped receiver can still take a type from outside.
func (c *checker) shiftShaped(n syntax.NodeID) bool {
	n = c.t.Unparen(n)
	if c.t.Kind(n) == syntax.Unary && c.t.View(n).AsUnary().Op().Kind() == token.Minus {
		return c.shiftShaped(c.t.View(n).AsUnary().Operand().ID())
	}
	if c.t.Kind(n) != syntax.Binary {
		return false
	}
	b := c.t.View(n).AsBinary()
	if op := b.Op().Kind(); shiftOp(op) {
		return true
	} else if shiftOperandOp(op) {
		return c.shiftShaped(b.Left().ID()) || c.shiftShaped(b.Right().ID())
	}
	return false
}

func (c *checker) shiftChecked(n syntax.NodeID, want TypeID) bool {
	op := c.t.View(n).Tok().Kind()
	return shiftOp(op) || shiftOperandOp(op) && c.r.Types.IsInteger(c.vars.resolve(want)) && c.shiftShaped(n)
}

// checkOperator pushes want into the operators that take their type from it.
func (c *checker) checkOperator(n syntax.NodeID, want TypeID) (TypeID, bool) {
	switch {
	case c.t.Kind(n) == syntax.Unary && isNumericLiteral(c.t, n), c.t.Kind(n) == syntax.Binary && c.t.View(n).Tok().Kind() == token.OrOr:
		return c.setType(n, c.synthKind(n, want)), true
	case c.t.Kind(n) == syntax.Binary && c.shiftChecked(n, want), c.negatedShift(n, want):
		return c.coerce(n, c.setType(n, c.synthKind(n, want)), want), true
	}
	return 0, false
}

func (c *checker) negatedShift(n syntax.NodeID, want TypeID) bool {
	return c.t.Kind(n) == syntax.Unary && c.shiftContext(want) != 0 && c.shiftShaped(n)
}

func (c *checker) shiftContext(t TypeID) TypeID {
	if t = c.vars.resolve(t); c.r.Types.IsInteger(t) {
		return t
	}
	return 0
}

// synthOperands types both operands; a shift among them takes the integer type its sibling or the expected result has, as a directly checked shift does.
func (c *checker) synthOperands(op token.Kind, lhs, rhs syntax.NodeID, want TypeID) (TypeID, TypeID) {
	read := func(t TypeID) TypeID { return c.readThrough(c.vars.resolve(t)) }
	shifted := func(n syntax.NodeID, ctx TypeID) TypeID {
		if ctx == 0 || c.info.Types[n] != 0 || !c.shiftShaped(n) {
			return read(c.synth(n))
		}
		return read(c.setType(n, c.synthKind(n, ctx)))
	}
	result := TypeID(0)
	if op != token.EqEq && op != token.NotEq && op != token.Lt && op != token.LtEq && op != token.Gt && op != token.GtEq {
		result = c.shiftContext(want)
	}
	switch {
	case !shiftOperandOp(op):
		return read(c.synth(lhs)), read(c.synth(rhs))
	case shiftOp(op):
		return shifted(lhs, result), read(c.synth(rhs))
	case c.shiftShaped(lhs) && !c.shiftShaped(rhs):
		r := read(c.synth(rhs))
		return shifted(lhs, cmp.Or(c.shiftContext(r), result)), r
	}
	l := shifted(lhs, result)
	return l, shifted(rhs, cmp.Or(result, c.shiftContext(l)))
}
