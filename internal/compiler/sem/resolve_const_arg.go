package sem

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// resolveConstLit turns a literal reached via the type grammar (e.g. `FixedArray[u8, 32]`), not
// a call's explicit type args, into a KConst value.
func (r *Result) resolveConstLit(f FileID, n syntax.NodeID) TypeID {
	t := r.tree(f)
	node := t.View(n)
	if node.Kind() == syntax.BoolLit {
		v := int64(0)
		if node.Tok().Kind() == token.KwTrue {
			v = 1
		}
		return r.Types.ConstVal(v, TyBool)
	}
	v, ok := syntax.ParseInt(node.Tok().Text())
	if !ok || !v.IsInt64() {
		r.errAt(f, n, cLiteralOutOfRange, node.Tok().Text(), TyUsize)
		return TyPoison
	}
	return r.Types.ConstVal(v.Int64(), TyUsize)
}

// `N + 1`-shaped expressions are out of scope for this phase.
func (r *Result) resolveConstArg(f FileID, scope ScopeID, n syntax.NodeID, want TypeID) TypeID {
	t := r.tree(f)
	switch t.Kind(n) {
	case syntax.Paren:
		return r.resolveConstArg(f, scope, t.View(n).AsParen().Inner().ID(), want)
	case syntax.IntLit, syntax.BoolLit:
		return r.coerceConstLit(f, n, want)
	case syntax.Unary:
		u := t.View(n).AsUnary()
		lhs := u.Operand().ID()
		if u.Op().Kind() == token.Minus && t.Kind(lhs) == syntax.IntLit {
			return r.coerceNegConstLit(f, n, lhs, want)
		}
		r.errAt(f, n, cConstExprUnsupported)
		return TyPoison
	case syntax.Ident:
		return r.constArgFromIdent(f, scope, n, n, want)
	case syntax.MemberExpr:
		path, ok := r.qualifiedConstPath(f, n)
		if !ok {
			r.errAt(f, n, cConstExprUnsupported)
			return TyPoison
		}
		ent := r.resolveValueName(f, scope, path)
		if ent == 0 {
			return TyPoison
		}
		r.Files[f].Uses[n] = ent
		return r.constArgFromEntity(f, n, ent, want)
	case syntax.TypePath:
		tp := t.View(n).AsTypePath()
		if tp.Args().Valid() {
			r.errAt(f, n, cConstExprUnsupported)
			return TyPoison
		}
		return r.constArgFromIdent(f, scope, tp.Path().ID(), n, want)
	}
	r.errAt(f, n, cConstExprUnsupported)
	return TyPoison
}

func (r *Result) constArgFromIdent(f FileID, scope ScopeID, path, n syntax.NodeID, want TypeID) TypeID {
	t := r.tree(f)
	var ent EntityID
	if t.Kind(path) == syntax.Ident {
		b, _, ok := r.lookup(scope, t.View(path).Tok().Text())
		if !ok || b.Ent == 0 {
			r.errAt(f, n, cUnresolvedName, t.View(path).Tok().Text())
			return TyPoison
		}
		ent = r.follow(b.Ent)
	} else {
		ent = r.resolveTypeName(f, scope, path)
		if ent == 0 {
			return TyPoison
		}
	}
	r.Files[f].Uses[n] = ent
	return r.constArgFromEntity(f, n, ent, want)
}

func (r *Result) constArgFromEntity(f FileID, n syntax.NodeID, ent EntityID, want TypeID) TypeID {
	switch r.Entities[ent].Kind {
	case EntTypeParam:
		info := r.typeParam(ent)
		if !info.IsConst {
			r.errAt(f, n, cConstArgKind, r.Entities[ent].Name, "a const", "a type")
			return TyPoison
		}
		if want != 0 && info.ConstType != want {
			r.errAt(f, n, cTypeMismatch, want, info.ConstType)
			return TyPoison
		}
		return r.Types.Param(ent)
	case EntConst:
		if r.comptimeConstUse(f, n, ent) {
			return TyPoison
		}
		v := r.evalConstDecl(ent)
		if v.Kind != constInt && v.Kind != constBool {
			r.errAt(f, n, cConstExprUnsupported)
			return TyPoison
		}
		return r.coerceConstValue(f, n, v, want)
	}
	r.errAt(f, n, cConstArgKind, r.Entities[ent].Name, "a const", "a type")
	return TyPoison
}
