package sem

import "kigumi/internal/compiler/syntax"

// discardedExpr is the expression of a statement whose value nothing keeps: `e` or `let _ = e`.
func (c *checker) discardedExpr(stmt syntax.NodeID) syntax.NodeID {
	switch c.t.Kind(stmt) {
	case syntax.ExprStmt:
		return c.t.View(stmt).AsExprStmt().Expr().ID()
	case syntax.LetStmt:
		l := c.t.View(stmt).AsLet()
		if c.t.Kind(l.Pattern().ID()) == syntax.PatWildcard && l.Else().ID() == 0 {
			return l.Init().ID()
		}
	}
	return 0
}

// A statement dropping `a.takeAt(0)` and mentioning a nowhere else empties the one-element array it was bound to.
func (c *checker) noteTakeAtKill(stmt syntax.NodeID) {
	if !c.topLevelStmt(stmt) {
		return
	}
	call := c.discardedExpr(stmt)
	if call == 0 {
		return
	}
	for local := range c.r.flow.soleElem {
		e := &c.r.Entities[local]
		if e.Parent != c.fn || !c.killableLocal(local) || !c.takesAtZero(call, e.Name) || !c.untouchedSince(local, stmt) {
			continue
		}
		c.addKill(local, c.r.containerField(fieldElement), stmt)
	}
}

// takesAtZero holds when the only mention of name in n is the receiver of a `takeAt(0)` call.
func (c *checker) takesAtZero(n syntax.NodeID, name string) bool {
	calls, mentions := 0, 0
	var walk func(x syntax.NodeID)
	walk = func(x syntax.NodeID) {
		if c.t.Kind(x) == syntax.Ident && c.t.View(x).Tok().Text() == name {
			mentions++
		}
		if c.t.Kind(x) == syntax.CallExpr {
			call := c.t.View(x).AsCallLike()
			if m := call.Callee().ID(); c.t.Kind(m) == syntax.MemberExpr && c.t.View(m).AsMember().Member().Text() == "takeAt" {
				if b := c.t.View(m).AsMember().Base().ID(); c.t.Kind(b) == syntax.Ident && c.t.View(b).Tok().Text() == name && call.Args().Len() == 1 && c.zeroIndex(call.Args().ItemID(0)) {
					calls++
				}
			}
		}
		for ch := range c.t.View(x).Children() {
			walk(ch.ID())
		}
	}
	walk(n)
	return calls == 1 && mentions == 1
}
