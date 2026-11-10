package sem

import (
	"math/big"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

func (r *Result) coerceConstLit(f FileID, n syntax.NodeID, want TypeID) TypeID {
	t := r.tree(f)
	node := t.View(n)
	if node.Kind() == syntax.BoolLit {
		if want != 0 && want != TyBool {
			r.errAt(f, n, cTypeMismatch, want, TyBool)
			return TyPoison
		}
		v := int64(0)
		if node.Tok().Kind() == token.KwTrue {
			v = 1
		}
		return r.Types.ConstVal(v, TyBool)
	}
	target := want
	if target == 0 {
		target = TyUsize
	}
	iv, ok := syntax.ParseInt(node.Tok().Text())
	if !ok {
		r.errAt(f, n, cLiteralOutOfRange, node.Tok().Text(), target)
		return TyPoison
	}
	if target == TyBool || !r.Types.IsInteger(target) {
		r.errAt(f, n, cTypeMismatch, target, TyUsize)
		return TyPoison
	}
	if !r.intFits(iv, target) {
		r.errAt(f, n, cLiteralOutOfRange, node.Tok().Text(), target)
		return TyPoison
	}
	v := iv.Int64()
	if !iv.IsInt64() {
		v = int64(iv.Uint64())
	}
	return r.Types.ConstVal(v, target)
}

// coerceNegConstLit evaluates a `-N` literal reached as a const generic argument.
func (r *Result) coerceNegConstLit(f FileID, n, lit syntax.NodeID, want TypeID) TypeID {
	t := r.tree(f)
	target := want
	if target == 0 {
		target = TyUsize
	}
	iv, ok := syntax.ParseInt(t.View(lit).Tok().Text())
	if !ok {
		r.errAt(f, n, cLiteralOutOfRange, "-"+t.View(lit).Tok().Text(), target)
		return TyPoison
	}
	iv = new(big.Int).Neg(iv)
	if target == TyBool || !r.Types.IsInteger(target) {
		r.errAt(f, n, cTypeMismatch, target, TyUsize)
		return TyPoison
	}
	if !r.intFits(iv, target) {
		r.errAt(f, n, cLiteralOutOfRange, iv.String(), target)
		return TyPoison
	}
	v := iv.Int64()
	if !iv.IsInt64() {
		v = int64(iv.Uint64())
	}
	return r.Types.ConstVal(v, target)
}

func (r *Result) coerceConstValue(f FileID, n syntax.NodeID, v constValue, want TypeID) TypeID {
	if v.Kind == constBool {
		if want != 0 && want != TyBool {
			r.errAt(f, n, cTypeMismatch, want, TyBool)
			return TyPoison
		}
		val := int64(0)
		if v.Bool {
			val = 1
		}
		return r.Types.ConstVal(val, TyBool)
	}
	target := want
	if target == 0 {
		target = TyUsize
	}
	if target == TyBool || !r.Types.IsInteger(target) {
		r.errAt(f, n, cTypeMismatch, target, TyUsize)
		return TyPoison
	}
	if !r.intFits(v.Int, target) {
		r.errAt(f, n, cLiteralOutOfRange, constIntText(v.Int), target)
		return TyPoison
	}
	val := v.Int.Int64()
	if !v.Int.IsInt64() {
		val = int64(v.Int.Uint64())
	}
	return r.Types.ConstVal(val, target)
}
