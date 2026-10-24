package sem

import (
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

func (c *checker) synthBinary(n syntax.NodeID, want TypeID) TypeID {
	bin := c.t.View(n).AsBinary()
	op := bin.Op().Kind()
	lhs, rhs := bin.Left().ID(), bin.Right().ID()
	switch op {
	case token.OrOr:
		return c.synthFallback(n, lhs, rhs, want)
	case token.PipeGt:
		return c.synthPipe(n, lhs, rhs)
	case token.AndAnd:
		c.scopedChain(n)
		return TyBool
	}
	l, r := c.synthOperands(op, lhs, rhs, want)
	tt := c.r.Types
	if op == token.DotDot || op == token.DotDotEq {
		return c.synthRange(n, op, lhs, rhs, l, r)
	}
	if folded := c.foldBinary(n, op, lhs, rhs, l, r); folded != 0 {
		return folded
	}
	if op == token.Shl || op == token.Shr {
		if folded := c.foldShift(n, op, lhs, rhs, l, r); folded != 0 {
			return folded
		}
	}
	switch op {
	case token.EqEq, token.NotEq, token.Lt, token.LtEq, token.Gt, token.GtEq:
		l, r = c.adoptPair(lhs, rhs, c.scrutineeOf(l), c.scrutineeOf(r))
		if l == TyPoison || r == TyPoison {
			return TyPoison
		}
		if op == token.EqEq || op == token.NotEq {
			return c.equality(n, lhs, rhs, l, r, op)
		}
		return c.ordering(n, rhs, l, r, op)
	}
	if op == token.Operator || tt.Kind(l) == KNamed || tt.Kind(r) == KNamed {
		return c.userOperator(n, bin.Op().Text(), []TypeID{l, r}, []syntax.NodeID{lhs, rhs})
	}
	if (op == token.Shl || op == token.Shr) && tt.Kind(r) == KUntyped {
		r = c.adopt(c.literalNode(rhs), TyUsize)
	}
	if (op == token.Shl || op == token.Shr) && r == TyUsize {
		l = c.shiftReceiver(lhs, l, want)
	}
	l, r = c.adoptPair(lhs, rhs, l, r)
	if l == TyPoison || r == TyPoison {
		return TyPoison
	}
	switch op {
	case token.Plus, token.Minus, token.Star, token.Slash, token.Percent:
		if l == r && tt.IsNumeric(l) {
			return c.builtinOp(n, op, l)
		}
	case token.Amp, token.Pipe, token.Caret:
		if l == r && tt.IsInteger(l) {
			return c.builtinOp(n, op, l)
		}
	case token.Shl, token.Shr:
		if tt.IsInteger(l) {
			if r != TyUsize {
				c.errAt(rhs, cShiftCountUsize, r)
				return TyPoison
			}
			return c.builtinOp(n, op, l)
		}
	}
	return c.operandMismatch(n, rhs, bin.Op().Text(), l, r)
}

// traps reports whether op's native codegen can panic on result's operands (overflow, division by zero, out-of-range shift).
func traps(op token.Kind, result TypeID, tt *TypeTable) bool {
	if !tt.IsInteger(result) {
		return false
	}
	switch op {
	case token.Plus, token.Minus, token.Star, token.Slash, token.Percent, token.Shl, token.Shr:
		return true
	}
	return false
}

func (c *checker) builtinOp(n syntax.NodeID, op token.Kind, result TypeID) TypeID {
	c.info.Calls[n] = CallInfo{Kind: CallBuiltinOp, Op: op, OpText: op.String()}
	if traps(op, result, c.r.Types) {
		c.edge(EffectEdge{Kind: EdgePanic, Node: n})
	}
	return result
}

func (c *checker) operandMismatch(n, rhs syntax.NodeID, op string, l, r TypeID) TypeID {
	tt := c.r.Types
	switch {
	case c.isMethodValue(rhs):
		c.mismatch(rhs, l, r)
	case tt.IsNumeric(l) && tt.IsNumeric(r) && l != r:
		c.errAtHelp(n, mixedUsizeHelp(l, r), cNumericMixed, l, r)
	case l != r && (tt.IsNumeric(l) || l == TyBool || l == TyString || l == TyChar || l == TyBytes):
		c.mismatch(rhs, l, r)
	default:
		c.errAt(n, cOperatorUndefined, op, c.r.TypeString(l)+", "+c.r.TypeString(r))
	}
	return TyPoison
}

func (c *checker) equality(n, lhs, rhs syntax.NodeID, l, r TypeID, op token.Kind) TypeID {
	if l != r && c.vars.unifiable(l, r) {
		c.vars.unify(l, r)
		l = c.vars.resolve(l)
		r = l
	}
	if l != r {
		return c.comparisonMismatch(n, rhs, l, r)
	}
	l = c.vars.settleLiterals(l)
	if !c.r.hasEq(l) {
		if why := c.r.protocolMismatch(l, "Eq"); why != "" {
			c.errAt(n, cProtocolMismatch, l, "equals", "Eq", why)
		} else {
			c.errAt(n, cEqUnsupported, l)
		}
		return TyPoison
	}
	if c.isBareNone(lhs) || c.isBareNone(rhs) {
		place := lhs
		if c.isBareNone(lhs) {
			place = rhs
		}
		fix := c.placeName(place) + " is None"
		if op == token.NotEq {
			fix = "!(" + fix + ")"
		}
		c.errAt(n, cEqNoneLint, fix)
	}
	from := len(*c.edges)
	if !c.primitiveCompare(l) && c.protoCall(n, l, "Eq", "equals", op) {
		c.borrowOperand(rhs, r)
		c.retagLendOperand(from, n, rhs)
		c.noteProtoSite(from, n, l, "equals", rhs)
		return TyBool
	}
	return c.builtinOp(n, op, TyBool)
}

func (c *checker) ordering(n, rhs syntax.NodeID, l, r TypeID, op token.Kind) TypeID {
	if l != r && c.vars.unifiable(l, r) {
		c.vars.unify(l, r)
		l = c.vars.resolve(l)
		r = l
	}
	if l != r {
		return c.comparisonMismatch(n, rhs, l, r)
	}
	l = c.vars.settleLiterals(l)
	if !c.r.hasOrd(l) {
		c.errAtHelp(n, c.orderingHelp(l), cOrdUnsupported, op.String(), l)
		return TyPoison
	}
	from := len(*c.edges)
	if !c.primitiveCompare(l) && c.protoCall(n, l, "Ord", "compareTo", op) {
		c.borrowOperand(rhs, r)
		c.retagLendOperand(from, n, rhs)
		return TyBool
	}
	return c.builtinOp(n, op, TyBool)
}

func (c *checker) orderingHelp(t TypeID) string {
	if n := c.r.Types.Node(t); n.Kind == KNamed && c.r.Types.isTupleEnt(n.Ent) {
		return "tuples have no ordering; compare the elements one by one, or use a record with `@impl(Ord)`"
	}
	return ""
}
