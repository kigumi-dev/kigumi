package sem

import "kigumi/internal/compiler/syntax"

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
	if e := &r.Entities[key.Ent]; e.Kind == EntLocal && e.Flags&EfMut == 0 || r.isResultEnt(key.Ent) {
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
		base := r.fnChainEnt(file, t.View(expr).AsMember().Base().ID())
		if base == 0 {
			return 0
		}
		return r.nestedFnOwner(fieldFnKey{Ent: base, Field: field})
	case syntax.BorrowExpr:
		return r.fnChainEnt(file, t.View(expr).AsBorrow().Operand().ID())
	case syntax.Paren:
		return r.fnChainEnt(file, t.View(expr).AsParen().Inner().ID())
	case syntax.CallExpr:
		if id := r.resultEnts[literalEntKey{File: file, Node: expr}]; id != 0 {
			return id
		}
		if _, ok := r.containerLitElems(file, expr); ok {
			return r.containerLitEnt(file, expr)
		}
	case syntax.TupleLit:
		return r.containerLitEnt(file, expr)
	case syntax.BracketExpr:
		return r.elementEnt(file, expr)
	case syntax.TryExpr:
		return r.tryPayloadEnt(file, expr)
	}
	return 0
}

func (r *Result) elementEnt(file FileID, expr syntax.NodeID) EntityID {
	t := r.tree(file)
	var base syntax.NodeID
	for ch := range t.View(expr).Children() {
		base = ch.ID()
		break
	}
	bt := r.Files[file].Types[base]
	if n := r.Types.Node(bt); n.Kind == KRef {
		bt = n.Elem
	}
	if n := r.Types.Node(bt); n.Kind != KNamed || (n.Ent != r.Types.arrayEnt || r.Types.arrayEnt == 0) && !r.isFixedArray(bt) {
		return 0
	}
	owner := r.fnChainEnt(file, base)
	if owner == 0 {
		return 0
	}
	return r.nestedFnOwner(fieldFnKey{Ent: owner, Field: r.containerField(fieldElement)})
}

// A copy of a local still written later is merged at the copy, since an alias would see those writes.
func (r *Result) writableLocalPlace(ent EntityID) bool {
	root := &r.Entities[r.rootOf(ent)]
	return root.Kind == EntLocal && root.Flags&EfMut != 0
}

// An immutable copy holds at most the fns its source ever holds.
func (c *checker) trackFnCopyAlias(init syntax.NodeID, local EntityID) {
	t := c.vars.resolve(c.r.Entities[local].Type)
	if c.r.Types.Kind(t) == KRef {
		t = c.r.Types.Node(t).Elem
	}
	if len(c.r.recordFieldsOf(t)) == 0 && !c.r.holdsFnElem(t) {
		return
	}
	src := c.r.fnChainEnt(c.f, init)
	if src == 0 || src == local || c.r.isLiteralFieldEnt(src) && !c.r.containerLits[src] {
		return
	}
	if c.r.writableLocalPlace(src) {
		c.unionInto(local, src, t, init)
		return
	}
	if c.writableParamPlace(src) {
		c.unionInto(local, src, t, init)
		c.notePlaceCopy(local, src)
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

// A copy of a `&mut` parameter's place holds what the callers bound plus the writes before the copy, so it merges at the copy and keeps the callers' binding.
func (c *checker) writableParamPlace(ent EntityID) bool {
	root := &c.r.Entities[c.r.rootOf(ent)]
	return len(c.lambdas) == 0 && root.Kind == EntParam && root.Parent == c.fn && root.Flags&EfMut == 0 && c.r.Types.RefMut(c.vars.resolve(root.Type))
}

func (r *Result) tryPayloadEnt(file FileID, expr syntax.NodeID) EntityID {
	operand := r.tree(file).View(expr).AsTry().Operand().ID()
	owner := r.fnChainEnt(file, operand)
	if owner == 0 {
		return 0
	}
	t := r.Files[file].Types[operand]
	if n := r.Types.Node(t); n.Kind == KRef {
		t = n.Elem
	}
	switch n := r.Types.Node(t); {
	case n.Kind != KNamed:
		return 0
	case n.Ent == r.Types.optionEnt:
		return r.nestedFnOwner(fieldFnKey{Ent: owner, Field: r.containerField(fieldPayload)})
	case n.Ent == r.Types.resultEnt && n.Ent != 0:
		return r.nestedFnOwner(fieldFnKey{Ent: owner, Field: r.containerField(fieldOk)})
	}
	return 0
}
