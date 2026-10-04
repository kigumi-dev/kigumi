package sem

import "kigumi/internal/syntax"

// definedUnderlying returns id's direct underlying type, unsubstituted; chain
// cycles are caught separately by checkDefinedChain.
func (r *Result) definedUnderlying(id EntityID) TypeID {
	info := r.typeDecl(id)
	if info.Underlying != 0 {
		return info.Underlying
	}
	e := &r.Entities[id]
	t := r.tree(e.File)
	s := typeDecl(t, e.Node)
	target := syntax.NodeID(t.Nodes[s.Body].Lhs)
	info.Underlying = r.resolveType(e.File, r.declScope(id), target, posDefined)
	return info.Underlying
}

// checkDefinedChain poisons every entity on a discovered Underlying cycle together.
func (r *Result) checkDefinedChain(id EntityID) {
	info := r.typeDecl(id)
	if info.Form != FormDefined || r.poisoned(id) {
		return
	}
	order := []EntityID{id}
	visited := map[EntityID]bool{id: true}
	cur := info.Underlying
	for {
		n := r.Types.Node(cur)
		if n.Kind != KNamed {
			return
		}
		next := n.Ent
		ninfo := r.typeDecl(next)
		if ninfo.Form != FormDefined {
			return
		}
		if visited[next] {
			e := &r.Entities[id]
			r.errAt(e.File, e.Node, cDefinedCycle, e.Name)
			for _, m := range order {
				r.typeDecl(m).Underlying = TyPoison
				r.Entities[m].Flags |= EfPoison
			}
			return
		}
		visited[next] = true
		order = append(order, next)
		cur = ninfo.Underlying
	}
}

// underlyingRepr resolves t through defined-type layers to its representation carrier.
func (r *Result) underlyingRepr(t TypeID) TypeID {
	n := r.Types.Node(t)
	switch n.Kind {
	case KRef:
		if elem := r.underlyingRepr(n.Elem); elem != n.Elem {
			return r.Types.Ref(elem, n.Flags&flagMut != 0)
		}
		return t
	case KNamed:
		info := r.typeDecl(n.Ent)
		if info.Form != FormDefined {
			return t
		}
		if info.Underlying == TyPoison {
			return TyPoison
		}
		return r.underlyingRepr(r.Types.Subst(info.Underlying, instSubst(info.Params, n.Args)))
	}
	return t
}

// definedAccessible reports whether a representation operation (wrap or `.value`) is usable from file f.
func (r *Result) definedAccessible(f FileID, ent EntityID) bool {
	return r.typeDecl(ent).Home == r.packageOf(f)
}

// definedValue checks `a.value`, the package-private unwrap projection onto
// n's receiver; it is a place like an ordinary field, not a method.
func (c *checker) definedValue(n syntax.NodeID, node typeNode) TypeID {
	ent := node.Ent
	if !c.r.definedAccessible(c.f, ent) {
		c.errAt(n, cDefinedValueOutside, c.r.Entities[ent].Name, c.r.Entities[ent].Name)
		return TyPoison
	}
	info := c.r.typeDecl(ent)
	if info.Underlying == TyPoison {
		return TyPoison
	}
	c.info.Uses[n] = ent
	c.placeOf(n)
	return c.r.Types.Subst(info.Underlying, instSubst(info.Params, node.Args))
}
