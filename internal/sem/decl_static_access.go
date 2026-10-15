package sem

import "kigumi/internal/syntax"

// checkStaticAccess requires unsafe for any read or write of a static mut or foreign extern static; assignStmt reaches it too since it synths its target through here.
func (c *checker) checkStaticAccess(n syntax.NodeID, ent EntityID, name string) {
	info := c.r.Static(ent)
	if !info.Mut && info.Owner != StaticExtern {
		return
	}
	if c.unsafe != 0 {
		return
	}
	what := "accessing `" + name + "`"
	if info.Mut {
		what = "accessing `static mut " + name + "`"
	}
	c.errAt(n, cUnsafeRequired, what)
}

// staticRootOf unwraps field/index projections to find the static an assignment or borrow ultimately targets.
func (c *checker) staticRootOf(n syntax.NodeID) (EntityID, bool) {
	for {
		switch c.t.Kind(n) {
		case syntax.Paren, syntax.MemberExpr, syntax.OptMemberExpr, syntax.BracketExpr:
			n = c.t.View(n).Base().ID()
			continue
		case syntax.Ident:
			ent := c.info.Uses[n]
			if ent != 0 && c.r.Entities[ent].Kind == EntStatic {
				return ent, true
			}
		}
		return 0, false
	}
}

// checkStaticAssign rejects any direct assignment to an immutable static; checkStaticAccess already gates the unsafe write to a static mut.
func (c *checker) checkStaticAssign(lhs syntax.NodeID, ent EntityID) {
	info := c.r.Static(ent)
	if info.Mut {
		return
	}
	name := c.r.Entities[ent].Name
	if info.Owner == StaticExtern {
		c.errAt(lhs, cStaticExternWrite, name)
		return
	}
	c.errAt(lhs, cStaticAssign, name)
}
