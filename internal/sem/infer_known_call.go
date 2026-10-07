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
		for key := range c.r.fieldParamEnts {
			if key.Ent == p.Root {
				c.poisonFieldKey(key)
			}
		}
	case 1:
		c.recordFieldWrite(p.Root, p.Fields[0], rhs)
	default:
		c.poisonFieldKey(fieldFnKey{Ent: p.Root, Field: p.Fields[0]})
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
		field := c.info.Uses[callee]
		if field == 0 || c.r.Entities[field].Kind != EntField {
			return nil
		}
		base := syntax.NodeID(c.t.Nodes[callee].Lhs)
		if c.t.Kind(base) != syntax.Ident {
			return nil
		}
		baseEnt := c.info.Uses[base]
		if baseEnt == 0 || (c.r.Entities[baseEnt].Kind != EntLocal && c.r.Entities[baseEnt].Kind != EntParam) {
			return nil
		}
		return c.r.knownFieldFn[fieldFnKey{Ent: baseEnt, Field: field}]
	}
	return nil
}

// knownFieldCallEdges binds each call-site argument's known fn to the field
// fn reads; n is EdgeFieldBind's Node, so forwardSubst can narrow per call site.
func (c *checker) knownFieldCallEdges(n syntax.NodeID, fn EntityID, args []syntax.NodeID) {
	info := c.r.Fn(fn)
	for _, key := range info.CalledFieldParams {
		i := slices.Index(info.Params, key.Param)
		if i < 0 || i >= len(args) || c.t.Kind(args[i]) == syntax.Spread {
			continue
		}
		dst := c.r.fieldParamEnt(fieldFnKey{Ent: key.Param, Field: key.Field})
		c.knownFieldArgEdge(n, args[i], key.Field, dst)
	}
}

// knownFieldArgEdge binds dst to arg's field: a concrete fn, or the
// caller's own field-param entity when arg just forwards its parameter.
func (c *checker) knownFieldArgEdge(n, arg syntax.NodeID, field, dst EntityID) {
	if c.t.Kind(arg) == syntax.RecordLit {
		for _, entry := range c.t.Children(syntax.NodeID(c.t.Nodes[arg].Rhs)) {
			if c.t.Kind(entry) == syntax.Spread || c.info.Uses[entry] != field {
				continue
			}
			for _, tgt := range c.knownFnRefs(syntax.NodeID(c.t.Nodes[entry].Lhs)) {
				c.edge(EffectEdge{Kind: EdgeFieldBind, Target: tgt, Param: dst, Node: n})
			}
			return
		}
		return
	}
	if c.t.Kind(arg) != syntax.Ident {
		return
	}
	ent := c.info.Uses[arg]
	if ent == 0 {
		return
	}
	switch c.r.Entities[ent].Kind {
	case EntParam:
		c.edge(EffectEdge{Kind: EdgeFieldBind, Target: c.r.fieldParamEnt(fieldFnKey{Ent: ent, Field: field}), Param: dst, Node: n})
	case EntLocal:
		for _, tgt := range c.r.knownFieldFn[fieldFnKey{Ent: ent, Field: field}] {
			c.edge(EffectEdge{Kind: EdgeFieldBind, Target: tgt, Param: dst, Node: n})
		}
	}
}
