package sem

import "kigumi/internal/syntax"

// whyDirectCross marks an EdgeAlloc for handing a direct aggregate to a
// position that holds a reference: the box is built there, not at the value.
const whyDirectCross = 5

// builtDirect reports that building t here costs nothing. An async body
// keeps every live value in a pointer-wide frame slot, so it builds the boxed form.
func (c *checker) builtDirect(t TypeID) bool {
	return c.r.DirectAgg(t) && !c.asyncBody()
}

// A lambda compiles to a body of its own, never into its creator's frame.
func (c *checker) asyncBody() bool {
	if len(c.lambdas) > 0 || c.r.Entities[c.fn].Kind != EntFn {
		return false
	}
	return c.r.Fn(c.fn).Declared&EffAsync != 0
}

func (c *checker) crossDirect(n syntax.NodeID, t TypeID) {
	t = c.vars.resolve(t)
	if c.r.DirectAgg(t) {
		c.edge(EffectEdge{Kind: EdgeAlloc, Node: n, Type: t, Why: whyDirectCross})
	}
}

// crossArg claims the box an argument needs when the parameter holds a
// reference: a type parameter, an interface, or an ABI fixed elsewhere.
func (c *checker) crossArg(a syntax.NodeID, p EntityID) {
	got := c.vars.resolve(c.info.Types[a])
	if p != 0 && c.r.DirectParamType(c.r.Entities[p].Parent, p) == got {
		return
	}
	c.crossDirect(a, got)
}

// crossAsyncResult claims the box a result needs on the way into an async
// frame, whose slots hold one pointer each.
func (c *checker) crossAsyncResult(n syntax.NodeID, ret TypeID) {
	if c.asyncBody() {
		c.crossDirect(n, ret)
	}
}

// crossRecv claims the box a receiver needs: a `mut self` method writes
// through the caller's cell, and a receiver already in one is handed on as is.
func (c *checker) crossRecv(n syntax.NodeID, fn EntityID, recv TypeID, recvNode syntax.NodeID) {
	if c.r.Fn(fn).Recv == RecvNone || c.r.DirectParamType(fn, c.r.Fn(fn).SelfParam) == recv {
		return
	}
	if recvNode != 0 && c.cellPlace(recvNode) {
		return
	}
	c.crossDirect(n, recv)
}

// crossStore claims the box a store needs when the place is someone else's
// storage: a container slot, a static, or a view a write reaches the owner through.
func (c *checker) crossStore(lhs syntax.NodeID, t TypeID) {
	if _, static := c.staticRootOf(lhs); static && c.t.Kind(lhs) == syntax.Ident {
		c.crossDirect(lhs, t)
		return
	}
	switch c.t.Kind(lhs) {
	case syntax.Ident:
		if c.cellPlace(lhs) {
			c.crossDirect(lhs, t)
		}
	case syntax.MemberExpr:
		base := syntax.NodeID(c.t.Nodes[lhs].Lhs)
		if c.cellPlace(base) || !c.r.DirectAgg(c.vars.resolve(c.info.Types[base])) {
			c.crossDirect(lhs, t)
			return
		}
		if c.t.Kind(base) != syntax.Ident {
			c.crossDirect(lhs, c.placeRootType(lhs))
		}
	default:
		c.crossDirect(lhs, t)
	}
}

// cellPlace reports a place the caller lends as a write-back cell, so a
// store into it reaches the cell's object rather than a value of this frame.
func (c *checker) cellPlace(n syntax.NodeID) bool {
	ent, ok := c.identPlaceEnt(n)
	return ok && (c.r.IsMutSelf(ent) || c.r.RefCellParam(ent))
}

// crossBorrow claims the box a reference needs: it names the owner's
// storage, which the unboxed form does not have.
func (c *checker) crossBorrow(inner syntax.NodeID) {
	c.crossDirect(inner, c.placeRootType(inner))
}

// placeRootType is the type of the value a projection chain starts from.
func (c *checker) placeRootType(n syntax.NodeID) TypeID {
	for i := 0; n != 0 && i < 64; i++ {
		switch c.t.Kind(n) {
		case syntax.MemberExpr, syntax.BracketExpr:
			n = syntax.NodeID(c.t.Nodes[n].Lhs)
		default:
			return c.vars.resolve(c.info.Types[n])
		}
	}
	return 0
}
