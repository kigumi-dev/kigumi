package sem

import "kigumi/internal/syntax"

func (c *checker) knownFieldArgEdge(n, arg syntax.NodeID, fk fieldFnKey) {
	dst := c.r.fieldParamEnt(fk)
	if c.t.Kind(arg) == syntax.RecordLit {
		for _, entry := range c.t.Children(syntax.NodeID(c.t.Nodes[arg].Rhs)) {
			if c.t.Kind(entry) == syntax.Spread || c.info.Uses[entry] != fk.Field {
				continue
			}
			c.bindFieldArgValue(n, syntax.NodeID(c.t.Nodes[entry].Lhs), fk, dst)
			return
		}
		c.poisonFieldKey(fk)
		return
	}
	if ent := c.r.fnChainEnt(c.f, arg); ent != 0 && !c.r.isLiteralFieldEnt(ent) {
		c.bindIdentFieldArg(n, ent, fk.Field, dst)
		for _, place := range c.r.paramPlacesOf(ent) {
			c.bindIdentFieldArg(n, place, fk.Field, dst)
		}
	}
}

// Anything but a record literal poisons fk; there is nothing left to defer to.
func (c *checker) bindFieldArgValue(n, init syntax.NodeID, fk fieldFnKey, dst EntityID) {
	if tgts := c.knownFnRefs(init); len(tgts) > 0 {
		for _, tgt := range tgts {
			c.edge(EffectEdge{Kind: EdgeFieldBind, Target: tgt, Param: dst, Node: n})
			c.r.noteEdgeSiteAt(c.f, n, tgt, dst, c.f, init)
		}
		return
	}
	if ent := c.identEnt(init); ent != 0 {
		c.bindIdentFieldArg(n, ent, fk.Field, dst)
		return
	}
	c.poisonFieldKey(fk)
}

func (c *checker) bindIdentFieldArg(n syntax.NodeID, ent, field, dst EntityID) {
	switch c.r.Entities[ent].Kind {
	case EntParam:
		c.noteFwdCalled(ent, field)
		c.edge(EffectEdge{Kind: EdgeFieldBind, Target: c.r.fieldParamEnt(fieldFnKey{Ent: ent, Field: field}), Param: dst, Node: n})
	case EntLocal, EntFieldParam:
		if c.r.Entities[ent].Kind == EntFieldParam {
			if place, ok := c.nestedPlaceOfParam(ent); ok {
				c.registerNestedCalled(place, field)
			}
			c.edge(EffectEdge{Kind: EdgeFieldBind, Target: c.r.fieldParamEnt(fieldFnKey{Ent: ent, Field: field}), Param: dst, Node: n})
		}
		srcKey := fieldFnKey{Ent: ent, Field: field}
		for _, tgt := range c.r.knownFieldFn[srcKey] {
			c.edge(EffectEdge{Kind: EdgeFieldBind, Target: tgt, Param: dst, Node: n})
			c.r.noteEdgeSite(srcKey, c.f, n, tgt, dst)
		}
		c.stageLateFieldEdge(EdgeFieldBind, srcKey, dst, n)
		c.r.fieldPoisonForwards = append(c.r.fieldPoisonForwards, fieldPoisonForward{src: srcKey, dst: dst, file: c.f, node: n})
	}
}
