package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

// pendingFieldEdge holds a field write's EdgeFieldBind/EdgeIfaceArg until
// flushFieldWrites, so a later unknown write to the key can still drop it.
type pendingFieldEdge struct {
	key    fieldFnKey
	kind   EdgeKind
	target EntityID
	node   syntax.NodeID
}

// recordFieldWrite is construction and reassignment's shared entry point: a
// fn- or interface-typed value joins the field's known set, anything else poisons it.
func (c *checker) recordFieldWrite(ent, field EntityID, init syntax.NodeID) {
	key := fieldFnKey{Ent: ent, Field: field}
	if ft := c.vars.resolve(c.r.Entities[field].Type); c.r.Types.Kind(ft) == KNamed && c.r.holdsFn(ft, 0) {
		c.recordNestedFnFields(key, init)
	}
	if c.r.fieldWitnessPoisoned[key] {
		return
	}
	c.r.fieldParamEnt(key) // indexes key for a later whole-value invalidation
	// field's declared type, not init's: a literal coerced into an
	// interface-typed field keeps its own concrete type in info.Types.
	t := c.vars.resolve(c.r.Entities[field].Type)
	switch c.r.Types.Kind(t) {
	case KFn, KClosure, KParam:
		// KParam: an Fn-bound or unconstrained type param both resolve the
		// same way, falling to a poison when the instantiation isn't a known fn.
		c.recordFnFieldWrite(key, init)
	case KIface:
		c.recordWitnessFieldWrite(key, init)
	case KNamed:
		c.recordNestedFieldWrite(key, init)
	}
}

// Anything but a record expr poisons the field.
func (c *checker) recordNestedFieldWrite(key fieldFnKey, init syntax.NodeID) {
	if c.r.Entities[key.Ent].Kind == EntLocal && c.r.Entities[key.Ent].Flags&EfMut == 0 {
		if src := c.fieldChainEnt(init); src != 0 {
			c.r.fieldWitnessOf[key] = src
			return
		}
	}
	c.poisonFieldKey(key)
}

// recordFnFieldWrite joins a fn-typed field write into knownFieldFn directly;
// a parameter defers to its own call-site binding (EdgeFieldBind).
func (c *checker) recordFnFieldWrite(key fieldFnKey, init syntax.NodeID) {
	if fns := c.knownFnRefs(init); len(fns) > 0 {
		c.r.joinKnownFieldFnAt(key, fns, fnSiteSrc{file: c.f, node: init})
		return
	}
	if ent := c.identEnt(init); ent != 0 && c.r.Entities[ent].Kind == EntParam {
		c.pendingFieldEdges = append(c.pendingFieldEdges, pendingFieldEdge{key, EdgeFieldBind, ent, init})
		return
	}
	if c.stageFnResultWrite(key, init) {
		return
	}
	c.recordFieldPoisonSite(key, init)
	c.poisonFieldKey(key)
}

type fieldPoisonSiteInfo struct {
	File FileID
	Node syntax.NodeID
}

func (c *checker) recordFieldPoisonSite(key fieldFnKey, init syntax.NodeID) {
	here := fieldPoisonSiteInfo{File: c.f, Node: init}
	keepEarlier(c.r, &c.r.fieldPoisonSite, key, here)
	keepEarlier(c.r, &c.r.flow.poisonAt, key, here)
}

// recordWitnessFieldWrite joins an interface-typed write into the field's
// witness set; an unresolved one still poisons, never left untracked.
func (c *checker) recordWitnessFieldWrite(key fieldFnKey, init syntax.NodeID) {
	if c.r.Entities[key.Ent].Kind == EntLocal && c.r.Entities[key.Ent].Flags&EfMut == 0 {
		if src := c.fieldChainEnt(init); src != 0 {
			c.r.fieldWitnessOf[key] = src
			return
		}
		c.poisonFieldKey(key)
		return
	}
	src := c.fieldChainEnt(init)
	if src == 0 {
		c.poisonFieldKey(key)
		return
	}
	if c.r.Entities[src].Kind == EntLocal {
		if w, ok := c.r.localWitness[src]; ok {
			c.r.joinLocalWitness(c.r.fieldParamEnt(key), w)
			return
		}
		c.poisonFieldKey(key)
		return
	}
	c.pendingFieldEdges = append(c.pendingFieldEdges, pendingFieldEdge{key, EdgeIfaceArg, src, init})
}

// fieldChainEnt resolves expr to the local/param it reads, the field-param
// entity for a nested read (`o.inner.c`), or a fresh witness entity for a literal.
func (c *checker) fieldChainEnt(expr syntax.NodeID) EntityID { return c.r.chainFieldEnt(c.f, expr) }

func (r *Result) joinLocalWitness(ent EntityID, add []EntityID) bool {
	changed := false
	for _, w := range add {
		if !slices.Contains(r.localWitness[ent], w) {
			r.localWitness[ent] = append(r.localWitness[ent], w)
			changed = true
		}
	}
	return changed
}

// poisonFieldKey wipes key's known set and blocks any later write from
// re-establishing it: a write's position in the source doesn't bound runtime order.
func (c *checker) poisonFieldKey(key fieldFnKey) {
	delete(c.r.knownFieldFn, key)
	delete(c.r.fieldWitnessOf, key)
	if ent, ok := c.r.fieldParamEnts[key]; ok {
		delete(c.r.localWitness, ent)
	}
	c.r.fieldWitnessPoisoned[key] = true
	if len(c.pendingFieldEdges) > 0 {
		for _, p := range c.pendingFieldEdges {
			if p.key == key {
				ent := c.r.fieldParamEnt(key)
				c.r.noteBindHist(ent, c.fn, EffectEdge{Kind: p.kind, Target: p.target, Param: ent, File: c.f, Node: p.node})
			}
		}
		c.pendingFieldEdges = slices.DeleteFunc(c.pendingFieldEdges, func(p pendingFieldEdge) bool { return p.key == key })
	}
}

// reassignRecordFields is applyFieldWrite's record-literal rhs case: a set
// field re-joins recordFieldWrite; a field left to a spread poisons instead.
func (c *checker) reassignRecordFields(root EntityID, lit syntax.NodeID) {
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
		c.recordFieldWrite(root, field, syntax.NodeID(c.t.Nodes[entry].Lhs))
	}
	if !spread {
		return
	}
	for key := range c.r.fieldParamEnts {
		if key.Ent == root && !set[key.Field] {
			c.poisonFieldKey(key)
		} else if top, ok := c.r.nestedTopField(key.Ent, root); ok && !set[top] && c.r.fnTypedFieldKey(key) {
			c.poisonFieldKey(key)
		}
	}
}

// flushFieldWrites emits each surviving pending field edge once the whole
// body is checked, so a write's own node order never matters.
func (c *checker) flushFieldWrites() {
	for _, p := range c.pendingFieldEdges {
		if c.r.fieldWitnessPoisoned[p.key] {
			continue
		}
		param := c.r.fieldParamEnt(p.key)
		c.edge(EffectEdge{Kind: p.kind, Target: p.target, Param: param, Node: p.node})
		c.r.noteFieldBindOwner(param, c.fn)
	}
	c.pendingFieldEdges = nil
}
