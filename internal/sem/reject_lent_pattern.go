package sem

import "kigumi/internal/syntax"

func (c *checker) noteLentBindings(p, scrutinee syntax.NodeID, info PatInfo, bound []EntityID) {
	head := c.t.Unparen(scrutinee)
	if c.t.Kind(head) == syntax.BorrowExpr {
		head = c.t.View(head).AsBorrow().Operand().ID()
	}
	root, field := c.placeRoot(head)
	if root == 0 {
		return
	}
	for _, ent := range bound {
		if c.r.Types.RefMut(c.vars.resolve(c.r.Entities[ent].Type)) {
			c.setLentFrom(ent, lentRef{root: root, field: field})
		}
	}
	if field == 0 && info.Kind == PatRecordTy && c.t.Kind(p) == syntax.PatRecord {
		c.noteRecordFieldBindings(p, root)
	}
}

// A record pattern splits the scrutinee: each binding owns only the field it was destructured from.
func (c *checker) noteRecordFieldBindings(p syntax.NodeID, root EntityID) {
	for f := range c.t.View(p).AsPatRecord().Fields().IDs() {
		if fld := c.info.Uses[f]; fld != 0 {
			c.fieldBindings(f, lentRef{root: root, field: fld})
		}
	}
}

func (c *checker) fieldBindings(n syntax.NodeID, from lentRef) {
	if ent := c.info.Defs[n]; ent != 0 {
		if _, lent := c.lentFrom[ent]; lent {
			c.setLentFrom(ent, from)
		}
	}
	for ch := range c.t.View(n).Children() {
		c.fieldBindings(ch.ID(), from)
	}
}

// A tuple literal destructured in place hands each element's `&mut` to the bindings at its position; a destructured holder lends its own place.
func (c *checker) noteTupleLent(p, expr syntax.NodeID) {
	expr = c.t.Unparen(expr)
	if c.t.Kind(p) == syntax.PatTuple && c.t.Kind(expr) == syntax.TupleLit {
		pats, items := c.t.List(p), c.t.List(expr)
		for i := 0; i < pats.Len() && i < items.Len(); i++ {
			c.noteTupleLent(pats.ItemID(i), items.ItemID(i))
		}
		return
	}
	if root, field := c.mutBorrowRoot(expr); root != 0 {
		c.lentBindingsIn(p, lentRef{root: root, field: field})
		return
	}
	if k := c.t.Kind(expr); k != syntax.Ident && k != syntax.MemberExpr {
		return
	}
	if root, field := c.placeRoot(expr); root != 0 {
		c.lentBindingsIn(p, lentRef{root: root, field: field})
		if field == 0 && c.t.Kind(p) == syntax.PatRecord {
			c.noteRecordFieldBindings(p, root)
		}
	}
}

func (c *checker) lentBindingsIn(n syntax.NodeID, from lentRef) {
	if ent := c.info.Defs[n]; ent != 0 && c.r.Entities[ent].Kind == EntLocal && c.r.Types.RefMut(c.vars.resolve(c.r.Entities[ent].Type)) {
		c.setLentFrom(ent, from)
	}
	for ch := range c.t.View(n).Children() {
		c.lentBindingsIn(ch.ID(), from)
	}
}
