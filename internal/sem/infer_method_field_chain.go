package sem

import "kigumi/internal/syntax"

// chainIdentEnt is identEnt's file-parameterized form: both the live checker
// and the post-pass method-call field-write summary need it.
func (r *Result) chainIdentEnt(file FileID, expr syntax.NodeID) EntityID {
	t := r.tree(file)
	if t.Kind(expr) != syntax.Ident {
		return 0
	}
	ent := r.Files[file].Uses[expr]
	if ent == 0 || (r.Entities[ent].Kind != EntLocal && r.Entities[ent].Kind != EntParam) {
		return 0
	}
	return r.canonEnt(ent)
}

func (r *Result) chainKnownFnRefs(file FileID, expr syntax.NodeID) []EntityID {
	t := r.tree(file)
	switch t.Kind(expr) {
	case syntax.Lambda:
		if cl := r.Files[file].Defs[expr]; cl != 0 {
			return []EntityID{cl}
		}
		return nil
	case syntax.Paren:
		return r.chainKnownFnRefs(file, t.View(expr).AsParen().Inner().ID())
	case syntax.MemberExpr:
		if call, ok := r.Files[file].Calls[expr]; ok && call.Kind == CallMethodValue {
			return []EntityID{call.Callee}
		}
		return nil
	case syntax.BracketExpr:
		if ent := r.Files[file].Uses[expr]; ent != 0 && r.Entities[ent].Kind == EntFn {
			return []EntityID{ent}
		}
		return nil
	case syntax.Ident:
	default:
		return nil
	}
	ent := r.Files[file].Uses[expr]
	if ent == 0 {
		return nil
	}
	switch r.Entities[ent].Kind {
	case EntFn:
		return []EntityID{ent}
	case EntLocal, EntParam:
		if fns := r.knownFn[ent]; len(fns) > 0 {
			return fns
		}
		if ty := r.Entities[ent].Type; r.Types.Kind(ty) == KClosure {
			return []EntityID{r.Types.Node(ty).Ent}
		}
	}
	return nil
}

// chainFieldEnt is fieldChainEnt's file-parameterized form.
func (r *Result) chainFieldEnt(file FileID, expr syntax.NodeID) EntityID {
	t := r.tree(file)
	switch t.Kind(expr) {
	case syntax.Ident:
		return r.chainIdentEnt(file, expr)
	case syntax.MemberExpr:
		return r.chainFieldWitnessRecvEnt(file, expr)
	case syntax.BorrowExpr:
		return r.chainFieldEnt(file, t.View(expr).AsBorrow().Operand().ID())
	case syntax.Paren:
		return r.chainFieldEnt(file, t.View(expr).AsParen().Inner().ID())
	}
	return r.literalFieldEnt(file, expr)
}

// literalFieldEnt mints (or returns the cached) EntFieldParam for a record
// literal's CoerceWitness, one per (file, node), so a fixpoint revisit never re-mints.
func (r *Result) literalFieldEnt(file FileID, expr syntax.NodeID) EntityID {
	key := literalEntKey{File: file, Node: expr}
	if id, ok := r.literalFieldEnts[key]; ok {
		return id
	}
	w, ok := r.Files[file].CoerceWitness[expr]
	if !ok {
		return 0
	}
	id := r.newEntity(Entity{Kind: EntFieldParam, Name: "<literal>", Pkg: r.Files[file].Pkg, File: file, Node: expr})
	r.localWitness[id] = w
	if r.literalFieldEnts == nil {
		r.literalFieldEnts = map[literalEntKey]EntityID{}
	}
	r.literalFieldEnts[key] = id
	return id
}

// literalEntKey names a record-literal node whose CoerceWitness minted a field-param entity.
type literalEntKey struct {
	File FileID
	Node syntax.NodeID
}

// chainFieldWitnessRecvEnt is fieldWitnessRecvEnt's file-parameterized form.
func (r *Result) chainFieldWitnessRecvEnt(file FileID, base syntax.NodeID) EntityID {
	field := r.Files[file].Uses[base]
	if field == 0 {
		return 0
	}
	baseEnt := r.chainFieldEnt(file, r.tree(file).View(base).AsMember().Base().ID())
	if baseEnt == 0 {
		return 0
	}
	return r.fieldKeyEnt(baseEnt, field)
}
