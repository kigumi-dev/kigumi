package sem

import "kigumi/internal/syntax"

// reinterprets an expression node as a type reference.
func (r *Result) exprAsTypeRef(f FileID, scope ScopeID, n syntax.NodeID) (EntityID, []TypeID, bool) {
	t := r.tree(f)
	node := t.View(n)
	switch node.Kind() {
	case syntax.Ident:
		b, _, ok := r.lookup(scope, node.Tok().Text())
		if !ok || b.Ent == 0 {
			r.errAt(f, n, cUnresolvedName, node.Tok().Text())
			return 0, nil, false
		}
		ent := r.follow(b.Ent)
		r.Files[f].Uses[n] = ent
		return ent, nil, true
	case syntax.MemberExpr:
		base := node.AsMember().Base()
		if base.Kind() != syntax.Ident {
			break
		}
		b, _, ok := r.lookup(scope, base.Tok().Text())
		if !ok || b.Ent == 0 || r.Entities[b.Ent].Kind != EntImport || r.Entities[b.Ent].Target == 0 ||
			r.Entities[r.Entities[b.Ent].Target].Kind != EntPackage {
			break
		}
		r.Entities[b.Ent].Flags |= EfUsed
		pkg := r.Entities[r.Entities[b.Ent].Target].Pkg
		member, ok := r.Scopes[r.Packages[pkg].Scope].Names[node.Tok().Text()]
		if !ok || member.Ent == 0 {
			r.errAt(f, n, cUnresolvedMember, r.Packages[pkg].Path, node.Tok().Text())
			return 0, nil, false
		}
		ent := r.follow(member.Ent)
		if !r.useEntity(f, n, ent) {
			return 0, nil, false
		}
		r.Files[f].Uses[n] = ent
		return ent, nil, true
	case syntax.BracketExpr:
		ent, _, ok := r.exprAsTypeRef(f, scope, node.AsBracket().Base().ID())
		if !ok {
			return 0, nil, false
		}
		var params []EntityID
		switch r.Entities[ent].Kind {
		case EntType:
			params = r.typeDecl(ent).Params
		case EntAlias:
			params = r.aliasParams[ent]
		}
		var args []TypeID
		for i, a := range node.AsBracket().Args().EnumerateIDs() {
			if k := t.Kind(a); i >= len(params) && (k == syntax.IntLit || k == syntax.BoolLit) {
				args = append(args, TyPoison)
				continue
			}
			args = append(args, r.resolveExprGenericArg(f, scope, a, entAt(params, i)))
		}
		return ent, args, true
	}
	r.errAt(f, n, cNotAType, r.exprName(f, n), "an expression")
	return 0, nil, false
}

// `T?` parses as TryExpr and becomes Option[T].
func (r *Result) exprAsType(f FileID, scope ScopeID, n syntax.NodeID) TypeID {
	t := r.tree(f)
	node := t.View(n)
	switch node.Kind() {
	case syntax.TryExpr:
		return r.Types.Option(r.exprAsType(f, scope, node.AsTry().Operand().ID()))
	case syntax.Paren:
		return r.exprAsType(f, scope, node.AsParen().Inner().ID())
	case syntax.TypeFn, syntax.TypePtr:
		return r.resolveType(f, scope, n, posTypeArg)
	case syntax.TupleLit:
		return r.exprAsTupleType(f, scope, n)
	case syntax.BorrowExpr:
		r.errAt(f, n, cBorrowPosition, typePosNames[posTypeArg])
		return TyPoison
	}
	ent, args, ok := r.exprAsTypeRef(f, scope, n)
	if !ok {
		return TyPoison
	}
	ty := r.instantiateTypeEntity(f, n, ent, args, posTypeArg)
	r.Files[f].Types[n] = ty
	return ty
}

// resolveExprGenericArg is exprAsType's counterpart to resolveGenericArg: a call type argument
// filling a const parameter (e.g. `zeroed[u8, 32]()`) gets const evaluation instead of an
// ordinary type.
func (r *Result) resolveExprGenericArg(f FileID, scope ScopeID, n syntax.NodeID, param EntityID) TypeID {
	if param != 0 {
		if info := r.typeParam(param); info.IsConst {
			return r.resolveConstArg(f, scope, n, info.ConstType)
		}
	}
	return r.exprAsType(f, scope, n)
}

// exprAsTupleType reinterprets a tuple literal `(a, b)` in a type-argument position as `(A, B)`,
// since a bare `(` can't be told from TypeTuple in the parser.
func (r *Result) exprAsTupleType(f FileID, scope ScopeID, n syntax.NodeID) TypeID {
	items := r.tree(f).List(n)
	ent, ok := r.Types.tupleEntity(items.Len())
	if !ok {
		r.errAt(f, n, cTupleArity, items.Len())
		for it := range items.IDs() {
			r.exprAsType(f, scope, it)
		}
		return TyPoison
	}
	args := make([]TypeID, items.Len())
	for i, it := range items.EnumerateIDs() {
		args[i] = r.exprAsType(f, scope, it)
	}
	ty := r.Types.Named(ent, args)
	r.Files[f].Types[n] = ty
	return ty
}
