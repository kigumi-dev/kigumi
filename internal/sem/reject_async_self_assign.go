package sem

import "kigumi/internal/syntax"

// The caller writes its receiver cell back when the call returns, which an async method's future outlives.
func (c *checker) rejectAsyncSelfAssign(lhs syntax.NodeID) {
	ent, ok := c.identPlaceEnt(lhs)
	if !ok {
		return
	}
	e := c.r.Entities[ent]
	if e.Flags&EfSelf == 0 || e.Flags&EfMove != 0 {
		return
	}
	owner := e.Parent
	if owner == 0 || c.r.Entities[owner].Kind != EntFn || c.r.Fn(owner).Declared&EffAsync == 0 {
		return
	}
	c.errAt(lhs, cAsyncSelfAssign, c.r.Entities[owner].Name)
}
