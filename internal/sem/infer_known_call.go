package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

// fieldParamEnt mints (or returns the cached) EntFieldParam standing for
// param's field, named for reporting exactly like a bound parameter.
func (r *Result) fieldParamEnt(key fieldFnKey) EntityID {
	if id, ok := r.fieldParamEnts[key]; ok {
		return id
	}
	p := &r.Entities[key.Ent]
	name := p.Name + "." + r.Entities[key.Field].Name
	id := r.newEntity(Entity{Kind: EntFieldParam, Name: name, Pkg: p.Pkg, File: p.File, Node: p.Node, Parent: key.Ent})
	r.fieldParamEnts[key] = id
	if r.fieldKeyByEnt == nil {
		r.fieldKeyByEnt = map[EntityID]fieldFnKey{}
	}
	r.fieldKeyByEnt[id] = key
	return id
}

// knownFnRefs is expr itself when it's a bare fn name, else what trackKnownFn
// already recorded for its local/param.
func (c *checker) knownFnRefs(expr syntax.NodeID) []EntityID { return c.r.chainKnownFnRefs(c.f, expr) }

// trackKnownFn records what local holds, so a later call through it can be traced.
func (c *checker) trackKnownFn(init syntax.NodeID, local EntityID) {
	if w := c.knownFnRefs(init); len(w) > 0 {
		c.r.knownFn[local] = w
	}
}

func (c *checker) recordKnownFieldFns(lit syntax.NodeID, local EntityID) {
	for _, entry := range c.t.Children(syntax.NodeID(c.t.Nodes[lit].Rhs)) {
		if c.t.Kind(entry) == syntax.Spread {
			continue
		}
		field := c.info.Uses[entry]
		if field == 0 {
			continue
		}
		c.recordFieldWrite(local, field, syntax.NodeID(c.t.Nodes[entry].Lhs))
	}
}

// identEnt is expr's own local/param entity, for a field init that just
// forwards a value through without calling it.
func (c *checker) identEnt(expr syntax.NodeID) EntityID { return c.r.chainIdentEnt(c.f, expr) }

// applyFieldWrite is assignStmt's `x.f = rhs` hook: a single-field write joins
// recordFieldWrite; anything else or a multi-hop place poisons conservatively.
func (c *checker) applyFieldWrite(lhs, rhs syntax.NodeID) {
	p, ok := c.buildPlace(lhs)
	if !ok {
		return
	}
	switch len(p.Fields) {
	case 0:
		if c.t.Kind(rhs) == syntax.RecordLit {
			c.reassignRecordFields(p.Root, rhs)
			return
		}
		if src := c.r.fnChainEnt(c.f, rhs); src != 0 && c.r.concretePlace(src) {
			c.unionInto(c.r.canonEnt(p.Root), src, c.r.Entities[p.Root].Type, rhs)
			return
		}
		for key := range c.r.fieldParamEnts {
			if key.Ent == p.Root || c.r.nestedUnder(key.Ent, p.Root) && c.r.fnTypedFieldKey(key) {
				c.poisonFieldKey(key)
			}
		}
	case 1:
		c.recordFieldWrite(c.r.canonEnt(p.Root), p.Fields[0], rhs)
	default:
		root := c.r.canonEnt(p.Root)
		c.poisonFieldKey(fieldFnKey{Ent: root, Field: p.Fields[0]})
		c.recordNestedPlaceWrite(root, p.Fields, rhs)
	}
}

// knownCallTargets is callValue's lookup for a call through a bare local or
// param (`f()`) or through a stored field (`b.f()`), to a concrete fn item.
func (c *checker) knownCallTargets(callee syntax.NodeID) []EntityID {
	if callee == 0 {
		return nil
	}
	switch c.t.Kind(callee) {
	case syntax.Ident:
		ent := c.info.Uses[callee]
		if ent == 0 {
			return nil
		}
		switch c.r.Entities[ent].Kind {
		case EntLocal, EntParam:
			return c.r.knownFn[ent]
		}
	case syntax.MemberExpr:
		if key, ok := c.knownFieldCallKey(callee); ok {
			return c.r.knownFieldFn[key]
		}
	}
	return nil
}

func (c *checker) knownFieldCallKeyOf(callee syntax.NodeID) (fieldFnKey, bool) {
	if callee == 0 || c.t.Kind(callee) != syntax.MemberExpr {
		return fieldFnKey{}, false
	}
	return c.knownFieldCallKey(callee)
}

func (c *checker) knownFieldCallKey(callee syntax.NodeID) (fieldFnKey, bool) {
	field := c.info.Uses[callee]
	if field == 0 || c.r.Entities[field].Kind != EntField {
		return fieldFnKey{}, false
	}
	base := syntax.NodeID(c.t.Nodes[callee].Lhs)
	switch c.t.Kind(base) {
	case syntax.Ident, syntax.MemberExpr, syntax.CallExpr:
		if baseEnt := c.r.fnChainEnt(c.f, base); baseEnt != 0 {
			return fieldFnKey{Ent: baseEnt, Field: field}, true
		}
	}
	return fieldFnKey{}, false
}

// knownFieldCallEdges binds each call-site argument's known fn to the field
// fn reads; n is EdgeFieldBind's Node, so forwardSubst can narrow per call site.
func (c *checker) knownFieldCallEdges(n syntax.NodeID, fn EntityID, args []syntax.NodeID) {
	info := c.r.Fn(fn)
	if c.fn != 0 {
		c.stageLateBindSite(n, fn, args, len(info.CalledFieldParams))
	}
	c.nestedArgEdges(n, fn, args, 0)
	for _, key := range info.CalledFieldParams {
		i := slices.Index(info.Params, key.Param)
		if i < 0 || i >= len(args) || c.t.Kind(args[i]) == syntax.Spread {
			continue
		}
		c.knownFieldArgEdge(n, args[i], fieldFnKey{Ent: key.Param, Field: key.Field})
	}
}
