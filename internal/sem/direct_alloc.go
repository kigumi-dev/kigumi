package sem

import "kigumi/internal/syntax"

// whyDirectCross marks an EdgeAlloc for handing a direct aggregate to a
// position that holds a reference: the box is built there, not at the value.
const whyDirectCross = 5

const whyGenericCross = 7

// whyLendCross is left to the built program, which finds out whether the callee takes the value.
const whyLendCross = 19

func directCrossWhy(why uint8) bool {
	return why == whyDirectCross || why == whyGenericCross || why == whyLendCross
}

func (c *checker) crossDirect(n syntax.NodeID, t TypeID) { c.crossDirectBut(n, t, 0, nil) }

// A type inference has not settled yet is decided by finish.
func (c *checker) crossDirectBut(n syntax.NodeID, t, keep TypeID, slot *EffectEdge) {
	t = c.vars.resolve(t)
	e := EffectEdge{Kind: EdgeAlloc, Node: n, Type: t, Why: whyDirectCross}
	if slot != nil {
		e.Why, e.Param, e.From = whyGenericCross, slot.Param, slot.From
	}
	if c.r.Types.ContainsVar(t) {
		c.edge(e)
		c.allocPending = append(c.allocPending, pendingAlloc{edges: c.edges, at: len(*c.edges) - 1, keep: keep})
		return
	}
	// A closure's captures settle only after checking, so checkEffects decides its edge.
	if t != keep && (c.r.DirectAgg(t) || c.r.containsClosure(t)) {
		c.edge(e)
	}
}

// crossArg claims the box an argument needs when the parameter holds a
// reference: a type parameter, an interface, or an ABI fixed elsewhere.
func (c *checker) crossArg(a syntax.NodeID, p EntityID) {
	if _, coerced := c.info.Coerce[a]; !coerced && c.nullaryValue(a) {
		return
	}
	keep := TypeID(0)
	var slot *EffectEdge
	if p != 0 {
		fn := c.r.Entities[p].Parent
		keep, slot = c.r.DirectParamType(fn, p), c.r.genericSlot(fn, c.r.Entities[p].Type)
	}
	c.crossDirectBut(a, c.argType(a), keep, slot)
}

// A coercion's last step names the type the argument arrives as, which an `Int` wrapped into `Int?` is not.
func (c *checker) argType(a syntax.NodeID) TypeID {
	if co, ok := c.info.Coerce[a]; ok && len(co.Steps) > 0 {
		return c.vars.resolve(co.Steps[len(co.Steps)-1].To)
	}
	return c.vars.resolve(c.info.Types[a])
}

// crossRecv claims the box a receiver needs: a `mut self` method writes
// through the caller's cell, and a receiver already in one is handed on as is.
func (c *checker) crossRecv(n syntax.NodeID, fn EntityID, recv TypeID, recvNode syntax.NodeID) {
	if c.r.Fn(fn).Recv == RecvNone || recvNode != 0 && c.cellPlace(recvNode) {
		return
	}
	self := c.r.Fn(fn).SelfParam
	c.crossDirectBut(n, recv, c.r.DirectParamType(fn, self), c.r.genericSlot(fn, c.r.Entities[self].Type))
}

// crossStore claims the box a store needs when the place is someone else's
// storage: a container slot, a static, or a view a write reaches the owner through.
func (c *checker) crossStore(lhs, rhs syntax.NodeID, t TypeID) bool {
	if _, coerced := c.info.Coerce[rhs]; !coerced && c.nullaryValue(rhs) {
		return false
	}
	// A `&mut T` place holds T; the reference is the cell the store reaches it through.
	if elem, ok := c.r.MutRefElem(c.vars.resolve(t)); ok {
		t = elem
	}
	if ent, static := c.staticRootOf(lhs); static && c.t.Kind(lhs) == syntax.Ident {
		if c.r.DirectStatic(ent) {
			return false
		}
		c.crossDirect(lhs, t)
		return true
	}
	switch c.t.Kind(lhs) {
	case syntax.Ident:
		if c.lentPlace(lhs) || c.sharedCellPlace(lhs) {
			c.crossDirect(lhs, t)
			return true
		}
	case syntax.MemberExpr, syntax.BracketExpr:
		base := c.t.View(lhs).Base().ID()
		if bt := c.vars.resolve(c.info.Types[base]); c.lentPlace(base) || !c.r.DirectAgg(bt) || c.r.containsClosure(bt) {
			c.crossDirect(lhs, t)
			return true
		}
		if c.t.Kind(base) != syntax.Ident {
			root := c.placeRootType(lhs)
			c.crossDirect(lhs, root)
			if c.foreignChain(base) || !c.r.DirectAgg(root) {
				c.crossDirect(lhs, t)
				return true
			}
		}
	default:
		c.crossDirect(lhs, t)
		return true
	}
	return false
}

// cellPlace reports a place the caller lends as a write-back cell, so a
// store into it reaches the cell's object rather than a value of this frame.
func (c *checker) cellPlace(n syntax.NodeID) bool {
	ent, ok := c.identPlaceEnt(n)
	return ok && (c.r.IsMutSelf(ent) || c.r.RefCellParam(ent))
}

// A local a closure writes or a borrow aliases lives in a cell, which holds the boxed form.
func (c *checker) sharedCellPlace(n syntax.NodeID) bool {
	ent, ok := c.identPlaceEnt(n)
	return ok && c.r.Entities[ent].Flags&(EfCapturedMut|EfAddrTaken) != 0
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
	n = c.t.Unparen(n)
	v := c.info.Uses[n]
	return v != 0 && c.r.Entities[v].Kind == EntVariant && len(c.r.variant(v).Payload) == 0
}

// nullaryShape is a name a payload-free variant may be written as, which only the checked value settles.
func (c *checker) nullaryShape(n syntax.NodeID) bool {
	k := c.t.Kind(c.t.Unparen(n))
	return k == syntax.Ident || k == syntax.MemberExpr
}

// foreignChain reports whether a projection chain reaches into storage a caller or a container owns, where a field or slot holds a reference.
func (c *checker) foreignChain(n syntax.NodeID) bool {
	for i := 0; n != 0 && i < 64; i++ {
		switch c.t.Kind(n) {
		case syntax.BracketExpr:
			return true
		case syntax.MemberExpr:
			n = c.t.View(n).AsMember().Base().ID()
		case syntax.Ident:
			_, static := c.staticRootOf(n)
			return static || c.lentPlace(n) || c.sharedCellPlace(n)
		default:
			return false
		}
	}
	return false
}

// placeRootType is the type of the value a projection chain starts from.
func (c *checker) placeRootType(n syntax.NodeID) TypeID {
	for i := 0; n != 0 && i < 64; i++ {
		switch c.t.Kind(n) {
		case syntax.MemberExpr, syntax.BracketExpr:
			n = c.t.View(n).Base().ID()
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
