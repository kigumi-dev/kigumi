package sem

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// untypedOps says which unary operators may join an untyped constant expression: `-` stays out of a float target,
// which keeps IEEE negative zero, and `~` out of an unsigned one, where its value depends on the width.
type untypedOps struct{ neg, tilde bool }

func (c *checker) untypedOpsFor(want TypeID) untypedOps {
	w := c.vars.resolve(want)
	tt := c.r.Types
	if tt.IsFloat(w) {
		return untypedOps{}
	}
	return untypedOps{neg: true, tilde: !tt.IsInteger(w) || tt.IsSigned(w)}
}

// untypedIntExpr holds for an expression of integer literals and untyped integer constants, which the const evaluator
// computes exactly before any type applies.
func (c *checker) untypedIntExpr(n syntax.NodeID, ops untypedOps) bool {
	v := c.t.View(n)
	switch v.Kind() {
	case syntax.IntLit:
		return true
	case syntax.Paren:
		return c.untypedIntExpr(v.AsParen().Inner().ID(), ops)
	case syntax.Unary:
		u := v.AsUnary()
		op := u.Op().Kind()
		return (op == token.Minus && ops.neg || op == token.Tilde && ops.tilde) && c.untypedIntExpr(u.Operand().ID(), ops)
	case syntax.Binary:
		b := v.AsBinary()
		return untypedIntOp(b.Op().Kind()) && c.untypedIntExpr(b.Left().ID(), ops) && c.untypedIntExpr(b.Right().ID(), ops)
	case syntax.Ident:
		_, ok := c.untypedConstNamed(n)
		return ok
	}
	return false
}

func untypedIntOp(op token.Kind) bool {
	switch op {
	case token.Plus, token.Minus, token.Star, token.Slash, token.Percent, token.Amp, token.Pipe, token.Caret, token.Shl, token.Shr:
		return true
	}
	return false
}

func compareOp(op token.Kind) bool {
	switch op {
	case token.EqEq, token.NotEq, token.Lt, token.LtEq, token.Gt, token.GtEq:
		return true
	}
	return false
}

func (c *checker) untypedConstNamed(n syntax.NodeID) (EntityID, bool) {
	if _, hole := c.info.ChainHoles[n]; hole {
		return 0, false
	}
	b, _, ok := c.r.lookup(c.scope, c.t.View(n).Tok().Text())
	if !ok || b.Ent == 0 {
		return 0, false
	}
	ent := c.r.follow(b.Ent)
	if c.r.Entities[ent].Kind != EntConst {
		return 0, false
	}
	v := c.r.evalConstDecl(ent)
	return ent, v.Kind == constInt && c.r.Entities[ent].Type == TyUntypedInt
}

// evalUntyped computes an untyped constant expression with the const evaluator's own operations and checks.
func (c *checker) evalUntyped(n syntax.NodeID) (constValue, bool) {
	v := c.t.View(n)
	switch v.Kind() {
	case syntax.IntLit:
		if c.literalValue(n) == TyPoison {
			return constValue{}, false
		}
		return c.info.Literals[n], true
	case syntax.Paren:
		return c.evalUntyped(v.AsParen().Inner().ID())
	case syntax.Ident:
		ent, _ := c.untypedConstNamed(n)
		c.info.Uses[n] = ent
		c.r.useEntity(c.f, n, ent)
		return c.r.constInfo(ent).Value, true
	case syntax.Unary:
		u := v.AsUnary()
		x, ok := c.evalUntyped(u.Operand().ID())
		if !ok {
			return x, false
		}
		return c.r.constUnary(c.f, n, u.Op().Kind(), x)
	}
	b := v.AsBinary()
	x, ok := c.evalUntyped(b.Left().ID())
	if !ok {
		return x, false
	}
	y, ok := c.evalUntyped(b.Right().ID())
	if !ok {
		return y, false
	}
	return c.r.constBinary(c.f, n, b.Op().Kind(), x, y)
}

// foldUntypedBinary evaluates an operator over untyped integer constants as a const item does; the result stays
// untyped, or is Bool for a comparison, and takes its type where it is used.
func (c *checker) foldUntypedBinary(n syntax.NodeID, want TypeID) (TypeID, bool) {
	b := c.t.View(n).AsBinary()
	op := b.Op().Kind()
	ops := c.untypedOpsFor(want)
	if compareOp(op) {
		ops = untypedOps{neg: true, tilde: true}
	} else if !untypedIntOp(op) {
		return 0, false
	}
	if !c.untypedIntExpr(b.Left().ID(), ops) || !c.untypedIntExpr(b.Right().ID(), ops) {
		return 0, false
	}
	v, ok := c.evalUntyped(n)
	switch {
	case !ok:
		return TyPoison, true
	case !compareOp(op) && !c.r.Types.IsInteger(c.vars.resolve(want)) && c.untypedUnaryIn(n):
		return c.foldAs(n, v, TyI64), true
	}
	c.info.Literals[n] = v
	return c.r.untypedOf(v), true
}

// untypedUnaryIn holds when a negation or `~` other than a negative literal takes part, which gives the expression Int
// unless an integer type is expected.
func (c *checker) untypedUnaryIn(n syntax.NodeID) bool {
	v := c.t.View(n)
	switch v.Kind() {
	case syntax.Paren:
		return c.untypedUnaryIn(v.AsParen().Inner().ID())
	case syntax.Unary:
		return !isNumericLiteral(c.t, n)
	case syntax.Binary:
		return c.untypedUnaryIn(v.AsBinary().Left().ID()) || c.untypedUnaryIn(v.AsBinary().Right().ID())
	}
	return false
}

func (c *checker) foldAs(n syntax.NodeID, v constValue, t TypeID) TypeID {
	if !c.r.intFits(v.Int, t) {
		c.errAt(n, cLiteralOutOfRange, constIntText(v.Int), t)
		return TyPoison
	}
	v.Typ = t
	c.info.Literals[n] = v
	return t
}

// foldUntypedUnary evaluates `-` or `~` of an untyped integer constant exactly and gives it the type an unfolded
// operator would take; a float or unsigned target keeps the run-time operator, which keeps IEEE negative zero.
func (c *checker) foldUntypedUnary(n syntax.NodeID, want TypeID) (TypeID, bool) {
	u := c.t.View(n).AsUnary()
	op := u.Op().Kind()
	if op != token.Minus && op != token.Tilde || !c.untypedIntExpr(u.Operand().ID(), untypedOps{neg: true, tilde: true}) {
		return 0, false
	}
	t := c.unaryLiteralTarget(TyUntypedInt, op, want)
	if !c.r.Types.IsInteger(t) || !c.r.Types.IsSigned(t) {
		return 0, false
	}
	v, ok := c.evalUntyped(n)
	if !ok {
		return TyPoison, true
	}
	return c.foldAs(n, v, t), true
}
