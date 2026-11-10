package sem

import (
	"cmp"
	"math/big"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// evalConstIn evaluates n as an expression that adopts want: with a float want, its untyped float operations round at
// that type, as the same expression does at run time once its literals take it.
func (r *Result) evalConstIn(f FileID, n syntax.NodeID, want TypeID) (constValue, bool) {
	if !r.Types.IsFloat(want) {
		want = 0
	}
	node := r.tree(f).View(n)
	switch node.Kind() {
	case syntax.Paren:
		return r.evalConstIn(f, node.AsParen().Inner().ID(), want)
	case syntax.Unary:
		v, ok := r.evalConstIn(f, node.AsUnary().Operand().ID(), want)
		if !ok {
			return v, false
		}
		if node.Tok().Kind() == token.Minus && v.Kind == constInt && v.Typ != 0 && !r.Types.IsSigned(v.Typ) {
			r.errAt(f, n, cUnaryMinusUnsigned, v.Typ)
			return constValue{}, false
		}
		return r.constUnary(f, n, node.Tok().Kind(), v)
	case syntax.Binary:
		return r.evalConstBinary(f, n, want)
	}
	return r.evalConst(f, n)
}

func (r *Result) evalConstBinary(f FileID, n syntax.NodeID, want TypeID) (constValue, bool) {
	bin := r.tree(f).View(n).AsBinary()
	op, lhs, rhs := bin.Op().Kind(), bin.Left().ID(), bin.Right().ID()
	if !floatArithOp(op) {
		want = 0
	}
	a, ok := r.evalConstIn(f, lhs, want)
	if !ok {
		return a, false
	}
	b, ok := r.evalConstIn(f, rhs, want)
	if !ok {
		return b, false
	}
	switch {
	case op == token.Shl || op == token.Shr || !isConstNumber(a) || !isConstNumber(b):
		return r.constBinary(f, n, op, a, b)
	case a.Kind == constInt && b.Kind == constInt:
		if _, ok := r.constOperandType(f, n, a, b); !ok {
			return constValue{}, false
		}
		return r.constBinary(f, n, op, a, b)
	}
	switch {
	case r.adoptsAt(b, a.Typ, want):
		b, ok = r.evalConstIn(f, rhs, a.Typ)
	case r.adoptsAt(a, b.Typ, want):
		a, ok = r.evalConstIn(f, lhs, b.Typ)
	}
	if !ok {
		return constValue{}, false
	}
	return r.constFloatBinary(f, n, op, a, b, want)
}

// adoptsAt reports an untyped float operand computed at want that must be recomputed at typ, its typed sibling's type.
func (r *Result) adoptsAt(v constValue, typ, want TypeID) bool {
	return v.Typ == 0 && v.Kind == constFloat && r.Types.IsFloat(typ) && floatPrec(r.Types, typ) != floatPrec(r.Types, want)
}

// constFloatBinary folds a numeric operation with a float operand. An untyped operand adopts the typed one's type;
// with neither typed, the operation rounds at want (Float when 0) and the result stays untyped.
func (r *Result) constFloatBinary(f FileID, n syntax.NodeID, op token.Kind, a, b constValue, want TypeID) (constValue, bool) {
	typ, ok := r.constOperandType(f, n, a, b)
	if !ok {
		return constValue{}, false
	}
	at := cmp.Or(typ, want, TyF64)
	if !floatArithOp(op) && !isComparison(op) || op == token.Percent {
		r.errAt(f, n, cOperatorUndefined, op.String(), r.TypeString(at)+", "+r.TypeString(at))
		return constValue{}, false
	}
	lhs, rhs := r.binaryOperands(f, n)
	x, okX := r.constFloatAt(f, lhs, a, at)
	y, okY := r.constFloatAt(f, rhs, b, at)
	if !okX || !okY {
		return constValue{}, false
	}
	if isComparison(op) {
		return constValue{Kind: constBool, Bool: compareResult(op, x.Cmp(y))}, true
	}
	if op == token.Slash && y.Sign() == 0 {
		r.errAt(f, n, cConstDivZero)
		return constValue{}, false
	}
	z, raw, _ := floatOp(r.Types, op, x, y, at)
	if z.IsInf() {
		r.errAt(f, n, cLiteralOutOfRange, raw.Text('g', 10), at)
		return constValue{}, false
	}
	return constValue{Kind: constFloat, Typ: typ, Float: z}, true
}

// constFloatAt is an operand's value in at's IEEE format, which must be finite.
func (r *Result) constFloatAt(f FileID, n syntax.NodeID, v constValue, at TypeID) (*big.Float, bool) {
	x := ieeeFloat(r.Types, toFloat(v), at)
	if x.IsInf() {
		r.errAt(f, n, cLiteralOutOfRange, toFloat(v).Text('g', -1), at)
		return nil, false
	}
	return x, true
}

func (r *Result) binaryOperands(f FileID, n syntax.NodeID) (syntax.NodeID, syntax.NodeID) {
	if t := r.tree(f); t.Kind(n) == syntax.Binary {
		return t.View(n).AsBinary().Left().ID(), t.View(n).AsBinary().Right().ID()
	}
	return n, n
}

// constOperandType is the type the operation runs at: the typed operand's, which the untyped one must be able to take.
func (r *Result) constOperandType(f FileID, n syntax.NodeID, a, b constValue) (TypeID, bool) {
	lhs, rhs := r.binaryOperands(f, n)
	switch {
	case a.Typ != 0 && b.Typ != 0 && a.Typ != b.Typ:
		if d, ok := r.diagAt(f, n, cNumericMixed, a.Typ, b.Typ); ok {
			if help := mixedUsizeHelp(a.Typ, b.Typ); help != "" {
				d.Help = []string{help}
			}
			r.emitDiag(f, d)
		}
		return 0, false
	case a.Typ != 0:
		return a.Typ, r.constTakes(f, rhs, b, a.Typ)
	case b.Typ != 0:
		return b.Typ, r.constTakes(f, lhs, a, b.Typ)
	}
	return 0, true
}

// constTakes reports whether v can become typ, with the literal diagnostics a run-time operand gets.
func (r *Result) constTakes(f FileID, n syntax.NodeID, v constValue, typ TypeID) bool {
	tt := r.Types
	switch {
	case v.Typ != 0:
	case v.Kind == constFloat && !tt.IsFloat(typ):
		r.errAt(f, n, cLiteralFloatIntoInt, v.Float.Text('g', -1), typ)
		return false
	case v.Kind == constFloat && !floatFits(v.Float, tt.Width(typ)):
		r.errAt(f, n, cLiteralOutOfRange, v.Float.Text('g', -1), typ)
		return false
	case v.Kind == constInt && tt.IsFloat(typ) && !intExactAsFloat(v.Int, tt.Width(typ)):
		r.errAt(f, n, cLiteralOutOfRange, v.Int.String(), typ)
		return false
	case v.Kind == constInt && tt.IsInteger(typ) && !r.intFits(v.Int, typ):
		r.errAt(f, n, cLiteralOutOfRange, v.Int.String(), typ)
		return false
	}
	return true
}
