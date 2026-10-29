package sem

import "kigumi/internal/syntax"

func (c *checker) hintFnParams(own []EntityID, inst []TypeID, subst map[EntityID]TypeID) {
	for i, p := range own {
		idx, ok := c.vars.index(inst[i])
		if !ok {
			continue
		}
		for _, con := range c.r.typeParam(p).Constraints {
			if con.Kind == CFn || con.Kind == CFnMut || con.Kind == CFnOnce {
				c.vars.hint[idx] = c.r.Types.Subst(con.Type, subst)
			}
		}
	}
}

// instantiateFn returns bad=true when the explicit type argument count was
// wrong, so own's instantiation is made up.
func (c *checker) instantiateFn(n syntax.NodeID, fn EntityID, typeArgs []TypeID, recv TypeID) (TypeID, []TypeID, bool) {
	info := c.r.Fn(fn)
	tt := c.r.Types
	subst := map[EntityID]TypeID{}
	ownerParams := c.r.ownerParams(fn)
	own := info.TypeParams
	if recv != 0 && len(ownerParams) > 0 && len(own) >= len(ownerParams) && own[0] == ownerParams[0] {
		rn := tt.Node(recv)
		var recvArgs []TypeID
		for i, p := range ownerParams {
			if i < len(rn.Args) {
				subst[p] = rn.Args[i]
				recvArgs = append(recvArgs, rn.Args[i])
			}
		}
		if len(info.RecvParams) > 0 {
			c.deferConstraints(n, info.RecvParams, recvArgs, subst)
		}
		own = own[len(ownerParams):]
	}
	bad := false
	if len(typeArgs) > 0 && len(typeArgs) != len(own) {
		c.errAt(n, cTypeArgCount, c.r.Entities[fn].Name, len(own), plural(len(own)), len(typeArgs))
		typeArgs = nil
		bad = true
	}
	var inst []TypeID
	for i, p := range own {
		var t TypeID
		switch {
		case i < len(typeArgs):
			t = typeArgs[i]
		case bad:
			t = TyPoison
		case c.r.typeParam(p).IsLifetime:
			t = c.vars.freshLifetime(n, p, c.lifetimeFallback(p))
		default:
			t = c.vars.fresh(n, c.r.Entities[p].Name)
		}
		subst[p] = t
		inst = append(inst, t)
	}
	if len(inst) > 0 && !bad {
		c.r.Instances[fn] = append(c.r.Instances[fn], Instance{Args: inst, File: c.f, Node: n, In: c.fn})
		c.deferConstraints(n, own, inst, subst)
		c.hintFnParams(own, inst, subst)
	}
	return tt.Subst(info.Sig, subst), inst, bad
}

// genericFnItemCallable reports whether n is a generic fn item (not yet
// resolved to a fixed signature) being matched against a callable type.
func (c *checker) genericFnItemCallable(n syntax.NodeID) (EntityID, bool) {
	item := c.info.Uses[n]
	if item == 0 || c.r.Entities[item].Kind != EntFn || len(c.ownGenerics(item)) == 0 {
		return 0, false
	}
	return item, true
}

// A bare type parameter never adopts a `&T` slot's referent: the item's body expects an owned parameter.
func (c *checker) coerceGenericFnItem(n syntax.NodeID, item EntityID, got, want TypeID) bool {
	var fresh []TypeID
	for i := len(c.r.Instances[item]) - 1; i >= 0; i-- {
		if c.r.Instances[item][i].Node == n {
			fresh = c.r.Instances[item][i].Args
			break
		}
	}
	tt := c.r.Types
	g, w := tt.Node(got), tt.Node(want)
	if fresh == nil || len(g.Args) != len(w.Args) || g.Flags&fnVariadic != w.Flags&fnVariadic ||
		(g.Flags^w.Flags)&uint16(EffUnsafe) != 0 {
		return false
	}
	mark := c.vars.mark()
	for i := range g.Args {
		if !c.vars.unifyIgnoringEscaping(g.Args[i], w.Args[i]) {
			c.vars.rollback(mark)
			return false
		}
	}
	for _, f := range fresh {
		if tt.Kind(c.vars.resolve(f)) == KRef {
			// Poison instead of unifying g.Elem/w.Elem: a rejected `&T` must not leak into the caller's U.
			c.vars.poison(w.Elem)
			c.errAt(n, cGenericCallableBorrow, c.r.Entities[item].Name)
			return true
		}
	}
	if !c.vars.unifyIgnoringEscaping(g.Elem, w.Elem) {
		c.vars.rollback(mark)
		return false
	}
	c.refScopeContract(n, got, want)
	if i, ok := c.firstEscapeMismatch(got, want); ok {
		c.errAt(n, cEscapingCoerce, c.typeText(got), i+1, c.typeText(want))
		return true
	}
	if need := Effects(w.Flags) & (EffPure | EffNoalloc); Effects(g.Flags)&need != need {
		c.edge(EffectEdge{Kind: EdgeContract, Target: item, Mods: need, Node: n})
	}
	c.noteNoallocErase(n, want)
	return true
}
