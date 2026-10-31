package sem

import "kigumi/internal/compiler/syntax"

// ctxKeyWriteRoot's whole is true when n is the key itself rather than a field or element of it.
func (c *checker) ctxKeyWriteRoot(n syntax.NodeID) (key EntityID, whole bool) {
	whole = true
	for {
		n = c.t.Unparen(n)
		switch c.t.Kind(n) {
		case syntax.Ident:
			if ent := c.info.Uses[n]; ent != 0 && c.r.Entities[ent].Kind == EntContextKey {
				return ent, whole
			}
			return 0, false
		case syntax.MemberExpr:
			ent := c.info.Uses[n]
			if ent != 0 && c.r.Entities[ent].Kind == EntContextKey {
				return ent, whole
			}
			if ent == 0 || c.r.Entities[ent].Kind != EntType {
				whole = false
			}
			n = c.t.View(n).AsMember().Base().ID()
		case syntax.BracketExpr:
			whole = false
			n = c.t.View(n).AsBracket().Base().ID()
		default:
			return 0, false
		}
	}
}

// rejectCtxKeyWrite: a callee holds a Copy key as a copy; an empty what means n is an assignment target.
func (c *checker) rejectCtxKeyWrite(n syntax.NodeID, what string) bool {
	key, whole := c.ctxKeyWriteRoot(n)
	if key == 0 {
		return false
	}
	switch {
	case what == "" && whole:
		c.errAt(n, cCtxKeyAssign, key, "assigned", ctxKeyWholeWhy)
		return true
	case what == "":
		what = "written through a field or element"
	}
	if !c.r.isCopy(c.r.Entities[key].Type) {
		return false
	}
	c.errAt(n, cCtxKeyAssign, key, what, ctxKeyCopyWhy)
	return true
}

// copyCtxKey: rejectCtxKeyWrite already reports every mutation through n.
func (c *checker) copyCtxKey(n syntax.NodeID) bool {
	key, _ := c.ctxKeyWriteRoot(n)
	return key != 0 && c.r.isCopy(c.r.Entities[key].Type)
}

// lentCtxKey is the key a `&mut key` argument lends whole; the cell a replacing callee writes back to is the lease, not the provider.
func (c *checker) lentCtxKey(a syntax.NodeID) EntityID {
	b := c.t.View(a).AsBorrow()
	if b.Flags()&syntax.FlagMut == 0 {
		return 0
	}
	key, whole := c.ctxKeyWriteRoot(b.Operand().ID())
	if key == 0 || !whole || c.r.isCopy(c.r.Entities[key].Type) {
		return 0
	}
	return key
}
