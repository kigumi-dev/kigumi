package sem

import "kigumi/internal/compiler/syntax"

var slotRoot = erasePos{pos: true, provable: true}

// argContracts holds each argument to the parameter type sig declares for it, and the receiver of a method fn to the
// receiver type it declares, under the call's substitution.
func (c *checker) argContracts(args []syntax.NodeID, sig TypeID, subst map[EntityID]TypeID, fn EntityID, recv syntax.NodeID) {
	for i := range args {
		if decl, inner := c.declaredArg(sig, args, i); decl != 0 {
			c.slotContracts(inner, decl, subst, slotRoot)
		}
	}
	c.recvContracts(recv, fn, subst)
}

// declaredArg is the parameter type argument i is held to and the node carrying the value: a variadic's element
// arguments each take its element type, and a spread takes the container and unwraps to the spread value.
func (c *checker) declaredArg(sig TypeID, args []syntax.NodeID, i int) (TypeID, syntax.NodeID) {
	n := c.r.Types.Node(sig)
	a := args[i]
	if c.t.Kind(a) == syntax.Spread {
		a = c.t.View(a).AsSpread().Value().ID()
	}
	if n.Flags&fnVariadic != 0 {
		fixed := len(n.Args) - 1
		if i < fixed {
			return n.Args[i], a
		}
		if c.t.Kind(args[i]) == syntax.Spread {
			return n.Args[fixed], a
		}
		return c.r.Types.Node(n.Args[fixed]).Args[0], a
	}
	if i < len(n.Args) {
		return n.Args[i], a
	}
	return 0, a
}

// recvContracts holds receiver recv of method fn, called or bound into a method value, to the receiver type fn declares.
func (c *checker) recvContracts(recv syntax.NodeID, fn EntityID, subst map[EntityID]TypeID) {
	if fn == 0 || recv == 0 || c.r.Fn(fn).SelfParam == 0 {
		return
	}
	t := c.r.Entities[c.r.Fn(fn).SelfParam].Type
	if c.r.Types.Kind(t) == KRef {
		t = c.r.Types.Node(t).Elem
	}
	c.slotContracts(recv, t, subst, slotRoot)
}

// fieldContracts holds a value assigned to field place lhs to the slot each enclosing record along the place declares
// for it, its type lifted from one record's parameters to the next.
func (c *checker) fieldContracts(lhs, rhs syntax.NodeID) {
	tt := c.r.Types
	var slot TypeID
	var inner EntityID
	for place := lhs; c.t.Kind(place) == syntax.MemberExpr; place = c.t.View(place).AsMember().Base().ID() {
		fld := c.info.Uses[place]
		base := c.vars.resolve(c.info.Types[c.t.View(place).AsMember().Base().ID()])
		if tt.Kind(base) == KRef {
			base = tt.Node(base).Elem
		}
		n := tt.Node(base)
		if fld == 0 || c.r.Entities[fld].Kind != EntField || n.Kind != KNamed || c.r.Entities[n.Ent].Kind != EntType {
			return
		}
		decl := c.r.typeDecl(n.Ent)
		if slot == 0 {
			slot = c.r.Entities[fld].Type
		} else if ft := tt.Node(c.r.Entities[fld].Type); ft.Kind == KNamed && ft.Ent == inner {
			slot = tt.Subst(slot, paramSubst(c.r.typeDecl(inner).Params, ft.Args))
		} else {
			return
		}
		c.slotContracts(rhs, slot, paramSubst(decl.Params, n.Args), slotRoot)
		inner = n.Ent
	}
}

func paramSubst(params []EntityID, args []TypeID) map[EntityID]TypeID {
	subst := map[EntityID]TypeID{}
	for i, p := range params {
		if i < len(args) {
			subst[p] = args[i]
		}
	}
	return subst
}

// markValueInstance marks the constraint checks since from as a value's instantiation: no call here supplies what the
// bound parameters will hold, so an erasure a bound needs is refused unless a call through the value fixes it.
func (c *checker) markValueInstance(from int) {
	for i := from; i < len(c.constraints); i++ {
		c.constraints[i].value = true
		if k, ok := c.vars.index(c.constraints[i].arg); ok {
			if c.vars.valueVars == nil {
				c.vars.valueVars = map[uint32]bool{}
			}
			c.vars.valueVars[k] = true
		}
	}
}

func (c *checker) valueErasure(cc constraintCheck, arg, bound TypeID) {
	if shape := c.valueShape(arg, bound); shape != 0 {
		if slots, ok := c.fitSlots(shape, bound); ok && len(slots) > 0 {
			c.refuseErasure(cc.node, c.valueItem(cc))
		}
	}
}

// valueItem is the fn a value instantiation is of: the bound parameter's own, or the item named at the value's node
// when the parameter belongs to the owner type.
func (c *checker) valueItem(cc constraintCheck) EntityID {
	for _, e := range []EntityID{c.r.Entities[cc.param].Parent, c.info.Uses[c.t.Unparen(cc.node)]} {
		if e != 0 && c.r.Entities[e].Kind == EntFn {
			return e
		}
	}
	return 0
}

// valueBoundWant binds a value instantiation's bounded variable param to its Fn bound when argument type got meets it
// only by an erasure, so the argument is coerced with its proof and later calls through the value meet the bound too.
func (c *checker) valueBoundWant(param, got TypeID) TypeID {
	tt := c.r.Types
	k, ok := c.vars.index(c.vars.resolve(param))
	if !ok || !c.vars.valueVars[k] || c.vars.hint[k] == 0 || tt.Kind(got) == KParam {
		return 0
	}
	hint := c.vars.resolve(c.vars.hint[k])
	if shape := c.valueShape(got, hint); shape != 0 {
		if slots, ok := c.fitSlots(shape, hint); ok && len(slots) > 0 && c.vars.unify(param, hint) {
			return hint
		}
	}
	return 0
}
