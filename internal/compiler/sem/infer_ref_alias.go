package sem

import "kigumi/internal/compiler/syntax"

func (c *checker) trackRefAlias(init syntax.NodeID, local EntityID) {
	if !c.r.Types.RefMut(c.vars.resolve(c.r.Entities[local].Type)) {
		return
	}
	src := c.fieldChainEnt(init)
	if src == 0 || src == local {
		return
	}
	if c.r.isLiteralFieldEnt(src) {
		return
	}
	if c.r.refAlias == nil {
		c.r.refAlias = map[EntityID]EntityID{}
	}
	c.r.refAlias[local] = src
}

func (r *Result) canonEnt(ent EntityID) EntityID {
	if a, ok := r.refAlias[ent]; ok {
		return a
	}
	return ent
}

func (r *Result) isLiteralFieldEnt(ent EntityID) bool {
	return r.Entities[ent].Kind == EntFieldParam && r.Entities[ent].Parent == 0 && !r.isResultEnt(ent)
}
