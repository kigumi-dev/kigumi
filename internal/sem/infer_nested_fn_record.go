package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

func (c *checker) recordNestedFnFields(key fieldFnKey, init syntax.NodeID) {
	for c.t.Kind(init) == syntax.Paren {
		init = syntax.NodeID(c.t.Nodes[init].Lhs)
	}
	if c.t.Kind(init) == syntax.RecordLit {
		c.recordNestedLiteral(key, init)
		return
	}
	if c.r.Entities[key.Ent].Kind == EntLocal && c.r.Entities[key.Ent].Flags&EfMut == 0 && c.fieldChainEnt(init) != 0 {
		return
	}
	if src := c.r.fnChainEnt(c.f, init); src != 0 && c.r.Entities[src].Kind == EntLocal && c.r.Entities[src].Flags&EfMut == 0 {
		c.copyNestedFnFields(src, c.r.nestedFnOwner(key), c.r.Entities[key.Field].Type, init, 0)
		return
	}
	c.poisonNestedFnFields(key, init, nil)
}

// A fn field the source never recorded may hold anything, so the copy poisons it.
func (c *checker) copyNestedFnFields(src, dst EntityID, t TypeID, site syntax.NodeID, depth int) {
	if depth >= maxNestedFnDepth {
		return
	}
	for _, f := range c.r.recordFieldsOf(t) {
		ft := c.r.Entities[f].Type
		from, to := fieldFnKey{Ent: src, Field: f}, fieldFnKey{Ent: dst, Field: f}
		if !c.r.fnTypedType(ft) {
			c.copyNestedFnFields(c.r.nestedFnOwner(from), c.r.nestedFnOwner(to), ft, site, depth+1)
			continue
		}
		if c.r.fieldWitnessPoisoned[to] {
			continue
		}
		if _, tracked := c.r.fieldParamEnts[from]; !tracked || c.r.fieldWitnessPoisoned[from] {
			c.recordFieldPoisonSite(to, site)
			c.poisonFieldKey(to)
			continue
		}
		c.r.fieldParamEnt(to)
		for _, fn := range c.r.knownFieldFn[from] {
			if !slices.Contains(c.r.knownFieldFn[to], fn) {
				c.r.knownFieldFn[to] = append(c.r.knownFieldFn[to], fn)
			}
		}
		for _, p := range slices.Clone(c.pendingFieldEdges) {
			if p.key == from {
				c.pendingFieldEdges = append(c.pendingFieldEdges, pendingFieldEdge{to, p.kind, p.target, p.node})
			}
		}
	}
}

func (c *checker) recordNestedLiteral(key fieldFnKey, lit syntax.NodeID) {
	owner := c.r.nestedFnOwner(key)
	set := map[EntityID]bool{}
	spread := false
	for _, entry := range c.t.Children(syntax.NodeID(c.t.Nodes[lit].Rhs)) {
		if c.t.Kind(entry) == syntax.Spread {
			spread = true
			continue
		}
		field := c.info.Uses[entry]
		if field == 0 {
			continue
		}
		set[field] = true
		if c.r.fnReachableType(c.r.Entities[field].Type) {
			c.recordFieldWrite(owner, field, syntax.NodeID(c.t.Nodes[entry].Lhs))
		}
	}
	if spread {
		c.poisonNestedFnFields(key, lit, set)
	}
}

func (r *Result) fnReachableType(t TypeID) bool { return r.holdsFn(t, 0) }

func (r *Result) holdsFn(t TypeID, depth int) bool {
	if r.fnTypedType(t) {
		return true
	}
	if depth >= maxNestedFnDepth {
		return false
	}
	for _, f := range r.recordFieldsOf(t) {
		if r.holdsFn(r.Entities[f].Type, depth+1) {
			return true
		}
	}
	return false
}

func (c *checker) poisonNestedFnFields(key fieldFnKey, site syntax.NodeID, skip map[EntityID]bool) {
	c.poisonNestedAt(c.r.nestedFnOwner(key), c.r.Entities[key.Field].Type, site, skip, 0)
}

func (c *checker) poisonNestedAt(owner EntityID, t TypeID, site syntax.NodeID, skip map[EntityID]bool, depth int) {
	if depth >= maxNestedFnDepth {
		return
	}
	for _, f := range c.r.recordFieldsOf(t) {
		if skip[f] {
			continue
		}
		ft := c.r.Entities[f].Type
		k := fieldFnKey{Ent: owner, Field: f}
		if c.r.fnTypedType(ft) {
			c.recordFieldPoisonSite(k, site)
			c.poisonFieldKey(k)
		} else {
			c.poisonNestedAt(c.r.nestedFnOwner(k), ft, site, nil, depth+1)
		}
	}
}

func (r *Result) nestedTopField(ent, root EntityID) (EntityID, bool) {
	for range maxNestedFnDepth * 2 {
		e := &r.Entities[ent]
		if e.Kind != EntFieldParam || e.Parent == 0 {
			return 0, false
		}
		if e.Parent == root {
			for key, id := range r.fieldParamEnts {
				if id == ent {
					return key.Field, true
				}
			}
			return 0, false
		}
		ent = e.Parent
	}
	return 0, false
}

func (r *Result) nestedUnder(ent, root EntityID) bool {
	_, ok := r.nestedTopField(ent, root)
	return ok
}

func (c *checker) recordNestedPlaceWrite(root EntityID, fields []EntityID, rhs syntax.NodeID) {
	last := fields[len(fields)-1]
	if !c.r.fnReachableType(c.r.Entities[last].Type) {
		return
	}
	owner := root
	for _, f := range fields[:len(fields)-1] {
		owner = c.r.nestedFnOwner(fieldFnKey{Ent: owner, Field: f})
	}
	c.recordFieldWrite(owner, last, rhs)
}

func (r *Result) applyNestedWriteSummary(site methodFieldCallSite, field, calleeFn, srcParam EntityID) bool {
	ft := r.Entities[field].Type
	if !r.holdsFn(ft, 0) || len(r.recordFieldsOf(ft)) == 0 {
		return false
	}
	srcNested, ok := r.fieldParamEnts[fieldFnKey{Ent: srcParam, Field: field}]
	if !ok {
		return false
	}
	inner := site
	inner.recv = r.nestedFnOwner(fieldFnKey{Ent: site.recv, Field: field})
	changed := false
	for _, f := range r.recordFieldsOf(ft) {
		if r.applyFieldWriteSummary(inner, f, calleeFn, srcNested) {
			changed = true
		}
	}
	return changed
}
