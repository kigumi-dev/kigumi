package sem

import "kigumi/internal/syntax"

const maxNestedFnDepth = 4

func (r *Result) fnTypedType(t TypeID) bool {
	switch r.Types.Kind(t) {
	case KFn, KClosure, KParam:
		return true
	}
	return false
}

func (r *Result) recordFieldsOf(t TypeID) []EntityID {
	ent := r.namedEnt(t)
	if ent == 0 || r.Entities[ent].Kind != EntType {
		return nil
	}
	return r.typeDecl(ent).Fields
}

func (r *Result) nestedFnOwner(key fieldFnKey) EntityID {
	if r.Entities[key.Ent].Kind == EntLocal && r.Entities[key.Ent].Flags&EfMut == 0 {
		if src := r.fieldWitnessOf[key]; src != 0 {
			return src
		}
	}
	return r.fieldParamEnt(key)
}

func (r *Result) fnIdentEnt(file FileID, expr syntax.NodeID) EntityID {
	ent := r.chainIdentEnt(file, expr)
	for range maxNestedFnDepth * 2 {
		next, ok := r.fnCopyAlias[ent]
		if !ok {
			break
		}
		ent = next
	}
	return ent
}

// chainFieldEnt would return 0 for a nested record built from a literal.
func (r *Result) fnChainEnt(file FileID, expr syntax.NodeID) EntityID {
	t := r.tree(file)
	switch t.Kind(expr) {
	case syntax.Ident:
		return r.fnIdentEnt(file, expr)
	case syntax.MemberExpr:
		field := r.Files[file].Uses[expr]
		if field == 0 || r.Entities[field].Kind != EntField {
			return 0
		}
		base := r.fnChainEnt(file, syntax.NodeID(t.Nodes[expr].Lhs))
		if base == 0 {
			return 0
		}
		return r.nestedFnOwner(fieldFnKey{Ent: base, Field: field})
	case syntax.BorrowExpr:
		return r.fnChainEnt(file, syntax.NodeID(t.Nodes[expr].Rhs))
	case syntax.Paren:
		return r.fnChainEnt(file, syntax.NodeID(t.Nodes[expr].Lhs))
	case syntax.CallExpr:
		return r.resultEnts[literalEntKey{File: file, Node: expr}]
	}
	return 0
}

// An immutable copy holds at most the fns its source ever holds, whenever the copy was made.
func (c *checker) trackFnCopyAlias(init syntax.NodeID, local EntityID) {
	t := c.vars.resolve(c.r.Entities[local].Type)
	if c.r.Types.Kind(t) == KRef {
		t = c.r.Types.Node(t).Elem
	}
	if len(c.r.recordFieldsOf(t)) == 0 {
		return
	}
	src := c.r.fnChainEnt(c.f, init)
	if src == 0 || src == local || c.r.isLiteralFieldEnt(src) {
		return
	}
	if c.r.fnCopyAlias == nil {
		c.r.fnCopyAlias = map[EntityID]EntityID{}
	}
	c.r.fnCopyAlias[local] = src
}

func (c *checker) aliasedParam(n syntax.NodeID) EntityID {
	if p := c.identParam(n); p != 0 {
		return p
	}
	if c.t.Kind(n) != syntax.Ident {
		return 0
	}
	src, ok := c.r.fnCopyAlias[c.info.Uses[n]]
	if !ok || c.r.Entities[src].Kind != EntParam || c.r.Entities[src].Parent != c.fn || c.r.Entities[src].Flags&EfMut != 0 {
		return 0
	}
	return src
}
