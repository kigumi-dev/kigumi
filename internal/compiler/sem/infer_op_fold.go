package sem

import (
	"math/big"
	"strings"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// Untyped literals adopt the other operand's type, defaulting if both are
// untyped; a pending range-element variable does the same but
// never floats, defaulting to Int when both sides are pending.
func (c *checker) adoptPair(lhs, rhs syntax.NodeID, l, r TypeID) (TypeID, TypeID) {
	tt := c.r.Types
	_, lp := c.vars.pendingIndex(l)
	_, rp := c.vars.pendingIndex(r)
	switch {
	case lp && rp:
		if c.vars.unify(l, TyI64) && c.vars.unify(r, TyI64) {
			l, r = c.vars.resolve(l), c.vars.resolve(r)
		}
	case lp:
		if v, pok := pendingRangeDefault(tt, r); pok && c.vars.unify(l, v) {
			l = c.vars.resolve(l)
		}
	case rp:
		if v, pok := pendingRangeDefault(tt, l); pok && c.vars.unify(r, v) {
			r = c.vars.resolve(r)
		}
	}
	lu, ru := tt.Kind(l) == KUntyped, tt.Kind(r) == KUntyped
	switch {
	case lu && ru:
		def := defaultOf(l)
		if l == TyUntypedFloat || r == TyUntypedFloat {
			def = TyF64
		}
		return c.adoptOperand(lhs, def), c.adoptOperand(rhs, def)
	case lu && tt.IsNumeric(r):
		return c.adoptOperand(lhs, r), r
	case ru && tt.IsNumeric(l):
		return l, c.adoptOperand(rhs, l)
	case lu:
		return c.adoptOperand(lhs, defaultOf(l)), r
	case ru:
		return l, c.adoptOperand(rhs, defaultOf(r))
	}
	return l, r
}

func pendingRangeDefault(tt *TypeTable, t TypeID) (TypeID, bool) {
	if tt.IsNumeric(t) {
		return t, true
	}
	if t == TyUntypedInt {
		return TyI64, true
	}
	return 0, false
}

// Under `&`, the literal inside adopts target and the borrow stays a borrow.
func (c *checker) adoptOperand(n syntax.NodeID, target TypeID) TypeID {
	n = c.literalNode(n)
	if c.t.Kind(n) != syntax.BorrowExpr {
		return c.adopt(n, target)
	}
	c.adopt(c.literalNode(c.t.View(n).AsBorrow().Operand().ID()), target)
	c.setType(n, c.r.Types.Ref(target, false))
	return target
}

// Returns 0 when not folded.
func (c *checker) foldBinary(n syntax.NodeID, op token.Kind, lhs, rhs syntax.NodeID, l, r TypeID) TypeID {
	tt := c.r.Types
	if tt.Kind(l) != KUntyped || tt.Kind(r) != KUntyped {
		return 0
	}
	lv, lok := c.info.Literals[c.literalNode(lhs)]
	rv, rok := c.info.Literals[c.literalNode(rhs)]
	if !lok || !rok {
		return 0
	}
	if lv.Kind == constInt && rv.Kind == constInt {
		if op == token.Star && constProductTooWide(lv.Int, rv.Int) {
			c.errAt(n, cConstTooLarge, maxConstBits)
			return TyPoison
		}
		out := new(big.Int)
		switch op {
		case token.Plus:
			out.Add(lv.Int, rv.Int)
		case token.Minus:
			out.Sub(lv.Int, rv.Int)
		case token.Star:
			out.Mul(lv.Int, rv.Int)
		case token.Slash, token.Percent:
			if rv.Int.Sign() == 0 {
				c.errAt(rhs, cConstDivZero)
				return TyPoison
			}
			if op == token.Slash {
				out.Quo(lv.Int, rv.Int)
			} else {
				out.Rem(lv.Int, rv.Int)
			}
		case token.Amp:
			out.And(lv.Int, rv.Int)
		case token.Pipe:
			out.Or(lv.Int, rv.Int)
		case token.Caret:
			out.Xor(lv.Int, rv.Int)
		default:
			return 0
		}
		if constTooWide(out) {
			c.errAt(n, cConstTooLarge, maxConstBits)
			return TyPoison
		}
		c.info.Literals[n] = constValue{Kind: constInt, Int: out}
		return TyUntypedInt
	}
	// Kigumi's float arithmetic is IEEE at runtime, so a folded expression must round like one.
	lf, rf := ieeeFloat(tt, toFloat(lv), 0), ieeeFloat(tt, toFloat(rv), 0)
	if lf.IsInf() || rf.IsInf() || op == token.Percent {
		return 0
	}
	if op == token.Slash && rf.Sign() == 0 {
		c.errAt(rhs, cConstDivZero)
		return TyPoison
	}
	out, raw, ok := floatOp(tt, op, lf, rf, 0)
	switch {
	case !ok:
		return 0
	case out.IsInf():
		c.errAt(n, cLiteralOutOfRange, raw.Text('g', 10), TyF64)
		return TyPoison
	}
	c.info.Literals[n] = constValue{Kind: constFloat, Float: out}
	return TyUntypedFloat
}

func (c *checker) compoundAssign(n, lhs, rhs syntax.NodeID, target TypeID) {
	opTok := c.t.View(n).Tok()
	op := opTok.Kind().BinaryOf()
	opText := strings.TrimSuffix(opTok.Text(), "=")
	r := c.vars.resolve(c.synth(rhs))
	target = c.vars.resolve(target)
	tt := c.r.Types
	if (op == token.Shl || op == token.Shr) && tt.Kind(r) == KUntyped {
		r = c.adopt(c.literalNode(rhs), TyUsize)
	}
	if target == TyPoison || r == TyPoison {
		return
	}
	var result TypeID
	if tt.Kind(target) == KNamed || tt.Kind(r) == KNamed {
		result = c.userOperator(n, opText, []TypeID{target, r}, []syntax.NodeID{lhs, rhs})
	} else {
		_, r = c.adoptPair(lhs, rhs, target, r)
		if r == TyPoison {
			return
		}
	}
	switch {
	case tt.Kind(target) == KNamed || tt.Kind(r) == KNamed:
	case (op == token.Shl || op == token.Shr) && tt.IsInteger(target):
		if r != TyUsize {
			c.errAt(rhs, cShiftCountUsize, r)
			return
		}
		if !c.checkConstCount(rhs, target) {
			return
		}
		result = target
	case target == r && (tt.IsInteger(target) || tt.IsNumeric(target) && op != token.Percent):
		result = target
	default:
		c.operandMismatch(n, rhs, opText, target, r)
		return
	}
	if result != TyPoison && result != target {
		c.errAt(n, cCompoundResultType, opText, target, r, result)
	}
	if _, user := c.info.Calls[n]; !user {
		c.info.Calls[n] = CallInfo{Kind: CallBuiltinOp, Op: op, OpText: opText}
	}
}
