package sem

import "kigumi/internal/syntax"

// whyDirectCross marks an EdgeAlloc for handing a direct aggregate to a
// position that holds a reference: the box is built there, not at the value.
const whyDirectCross = 5

// A lambda compiles to a body of its own, never into its creator's frame.
func (c *checker) asyncBody() bool {
	if len(c.lambdas) > 0 || c.r.Entities[c.fn].Kind != EntFn {
		return false
	}
	return c.r.Fn(c.fn).Declared&EffAsync != 0
}

func (c *checker) crossDirect(n syntax.NodeID, t TypeID) { c.crossDirectBut(n, t, 0) }

// A type inference has not settled yet is decided by finish.
func (c *checker) crossDirectBut(n syntax.NodeID, t, keep TypeID) {
	t = c.vars.resolve(t)
	if c.r.Types.ContainsVar(t) {
		c.edge(EffectEdge{Kind: EdgeAlloc, Node: n, Type: t, Why: whyDirectCross})
		c.allocPending = append(c.allocPending, pendingAlloc{edges: c.edges, at: len(*c.edges) - 1, keep: keep})
		return
	}
	if t != keep && c.r.DirectAgg(t) {
		c.edge(EffectEdge{Kind: EdgeAlloc, Node: n, Type: t, Why: whyDirectCross})
	}
}

// crossArg claims the box an argument needs when the parameter holds a
// reference: a type parameter, an interface, or an ABI fixed elsewhere.
func (c *checker) crossArg(a syntax.NodeID, p EntityID) {
	if _, coerced := c.info.Coerce[a]; !coerced && c.nullaryValue(a) {
		return
	}
	keep := TypeID(0)
	if p != 0 {
		keep = c.r.DirectParamType(c.r.Entities[p].Parent, p)
	}
	c.crossDirectBut(a, c.argType(a), keep)
}

// A coercion's last step names the type the argument arrives as, which an `Int` wrapped into `Int?` is not.
func (c *checker) argType(a syntax.NodeID) TypeID {
	if co, ok := c.info.Coerce[a]; ok && len(co.Steps) > 0 {
		return c.vars.resolve(co.Steps[len(co.Steps)-1].To)
	}
	return c.vars.resolve(c.info.Types[a])
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
	if c.r.Fn(fn).Recv == RecvNone || recvNode != 0 && c.cellPlace(recvNode) {
		return
	}
	c.crossDirectBut(n, recv, c.r.DirectParamType(fn, c.r.Fn(fn).SelfParam))
}

// crossStore claims the box a store needs when the place is someone else's
// storage: a container slot, a static, or a view a write reaches the owner through.
func (c *checker) crossStore(lhs syntax.NodeID, t TypeID) {
	// A `&mut T` place holds T; the reference is the cell the store reaches it through.
	if elem, ok := c.r.MutRefElem(c.vars.resolve(t)); ok {
		t = elem
	}
	if _, static := c.staticRootOf(lhs); static && c.t.Kind(lhs) == syntax.Ident {
		c.crossDirect(lhs, t)
		return
	}
	switch c.t.Kind(lhs) {
	case syntax.Ident:
		if c.lentPlace(lhs) {
			c.crossDirect(lhs, t)
		}
	case syntax.MemberExpr:
		base := syntax.NodeID(c.t.Nodes[lhs].Lhs)
		if c.lentPlace(base) || !c.r.DirectAgg(c.vars.resolve(c.info.Types[base])) {
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

// The store side runs while refCellParams is still being filled.
func (c *checker) lentPlace(n syntax.NodeID) bool {
	ent, ok := c.identPlaceEnt(n)
	return ok && (c.r.IsMutSelf(ent) || c.r.wholeRefBinding(c.r.Entities[ent].Type))
}

// crossBorrow claims the box a reference needs: it names the owner's
// storage, which the unboxed form does not have.
func (c *checker) crossBorrow(inner syntax.NodeID) {
	if !c.nullaryValue(inner) {
		c.crossDirect(inner, c.placeRootType(inner))
	}
}

// A payload-free variant's boxed form is the one immortal every site reads, so lending it costs nothing.
func (c *checker) nullaryValue(n syntax.NodeID) bool {
	for c.t.Kind(n) == syntax.Paren {
		n = syntax.NodeID(c.t.Nodes[n].Lhs)
	}
	v := c.info.Uses[n]
	return v != 0 && c.r.Entities[v].Kind == EntVariant && len(c.r.variant(v).Payload) == 0
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

func (c *checker) borrowOperand(rhs syntax.NodeID, t TypeID) {
	if c.r.Types.Kind(c.vars.resolve(t)) != KRef {
		c.crossBorrow(rhs)
	}
}
