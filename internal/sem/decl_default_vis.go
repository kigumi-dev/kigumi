package sem

import "kigumi/internal/syntax"

// checkDefaultVisibility rejects a default that names a declaration less visible than the function it belongs to.
func (c *checker) checkDefaultVisibility(thunk EntityID) {
	if c.r.Fn(thunk).DefaultOf == 0 {
		return
	}
	fn := c.r.Entities[c.r.Fn(thunk).DefaultOf]
	var walk func(v syntax.View)
	walk = func(v syntax.View) {
		n := v.ID()
		for _, ent := range c.namedDecls(n) {
			e := &c.r.Entities[ent]
			if e.Pkg != 0 && !c.r.covers(e.Vis, fn.Vis) {
				c.errAt(n, cDefaultNotAllowed, "a function whose default uses the less visible `"+e.Name+"`")
				return
			}
		}
		for child := range v.Children() {
			walk(child)
		}
	}
	for stmt := range c.t.List(c.r.Fn(thunk).Body).Views() {
		walk(stmt)
	}
}

func (c *checker) namedDecls(n syntax.NodeID) []EntityID {
	var out []EntityID
	if int(n) < len(c.info.Uses) {
		if ent := c.info.Uses[n]; ent != 0 && declaredItem(c.r.Entities[ent].Kind) {
			out = append(out, ent)
		}
	}
	if call, ok := c.info.Calls[n]; ok && call.Callee != 0 && declaredItem(c.r.Entities[call.Callee].Kind) {
		out = append(out, call.Callee)
	}
	return out
}

func declaredItem(k EntityKind) bool {
	switch k {
	case EntFn, EntConst, EntStatic, EntType, EntAlias:
		return true
	}
	return false
}
