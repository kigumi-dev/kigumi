package sem

import "kigumi/internal/syntax"

func (c *checker) noteClosureResult(call syntax.NodeID, closure EntityID) {
	ent := &c.r.Entities[closure]
	if ent.File != c.f || c.t.Kind(ent.Node) != syntax.Lambda {
		return
	}
	ret := c.vars.resolve(c.r.closure(closure).Ret)
	if c.r.resultRecordEnt(ret) == 0 {
		return
	}
	res := c.r.callResultEnt(c.f, call, closure)
	for _, leaf := range c.tailLeaves(syntax.NodeID(c.t.Nodes[ent.Node].Rhs), 0) {
		if lit := c.stripParen(leaf); c.t.Kind(lit) == syntax.RecordLit {
			c.recordKnownFieldFns(lit, res)
		} else if src := c.r.fnChainEnt(c.f, leaf); src != 0 && c.r.concretePlace(src) {
			c.unionInto(res, src, ret, call)
		}
	}
}
