package sem

import "kigumi/internal/syntax"

// synthBracket implements `a[b]`.
func (c *checker) synthBracket(n syntax.NodeID) TypeID {
	node := c.t.Nodes[n]
	base := syntax.NodeID(node.Lhs)
	items := c.t.Children(syntax.NodeID(node.Rhs))
	head := c.resolveHead(base)
	switch head.kind {
	case headFn:
		if params := c.ownGenerics(head.ent); len(params) > 0 {
			if c.r.Types.Node(c.r.Fn(head.ent).Sig).Flags&fnVariadic != 0 {
				name := c.r.Entities[head.ent].Name
				c.errAt(n, cVariadicNoValue, name, name)
				return TyPoison
			}
			c.rejectHiddenArgs(n, head.ent)
			var typeArgs []TypeID
			for i, a := range items {
				typeArgs = append(typeArgs, c.r.resolveExprGenericArg(c.f, c.scope, a, entAt(params, i)))
			}
			c.r.useEntity(c.f, n, head.ent)
			c.info.Uses[n] = head.ent
			sig, inst, _ := c.instantiateFn(n, head.ent, typeArgs, 0)
			c.errorAsCheck(n, head.ent, inst)
			c.deferFFIValueCheck(n, head.ent, inst, 0)
			c.touched = append(c.touched, n)
			if c.r.Fn(head.ent).Declared&EffAsync != 0 {
				sig = c.asyncValueFn(sig)
			}
			c.recordCtxValueRef(n, head.ent)
			return sig
		}
	case headType:
		if len(c.r.typeDecl(head.ent).Params) > 0 {
			c.errAt(n, cTypeAsValue, c.r.Entities[head.ent].Name)
			return TyPoison
		}
	case headSet:
		c.errAt(n, cOverloadValueAmbig, c.r.Overloads[head.set].Name)
		return TyPoison
	case headNone:
		return TyPoison
	}
	recv := c.vars.resolve(c.synthPlace(base))
	if len(items) != 1 {
		c.errAt(n, cIndexCount)
		return TyPoison
	}
	index := c.vars.resolve(c.synth(items[0]))
	return c.indexType(n, items[0], recv, index)
}

// indexValueResult gates a by-value index read on T: Copy. n is exempt when it was
// reached as a place (synthPlace) to extend further, not to materialize a value.
func (c *checker) indexValueResult(n syntax.NodeID, elem TypeID) TypeID {
	if c.indexAsPlace[n] {
		delete(c.indexAsPlace, n)
		return elem
	}
	if !c.r.isCopy(c.vars.resolve(elem)) {
		c.errAt(n, cIndexValueCopy, elem)
	}
	return elem
}

// checkIndexFieldValue extends indexValueResult to a field read whose base is
// (transitively) an index: `xs[i].field` is as untracked as `xs[i]` itself.
func (c *checker) checkIndexFieldValue(n syntax.NodeID, t TypeID) TypeID {
	if t == 0 || t == TyPoison || !c.indexRooted(n) || c.r.isCopy(c.vars.resolve(t)) {
		return t
	}
	c.errAt(n, cIndexValueCopy, t)
	return t
}

// indexRooted reports whether n bottoms out, through parens and field projections,
// at a bracket index. Walks the syntax directly so an unresolvable base can't hide it.
func (c *checker) indexRooted(n syntax.NodeID) bool {
	for {
		switch c.t.Kind(n) {
		case syntax.Paren, syntax.MemberExpr, syntax.OptMemberExpr:
			n = syntax.NodeID(c.t.Nodes[n].Lhs)
		case syntax.BracketExpr:
			return true
		default:
			return false
		}
	}
}

func (c *checker) ownGenerics(fn EntityID) []EntityID {
	own := c.r.Fn(fn).TypeParams
	if n := len(c.r.ownerParams(fn)); n > 0 && len(own) >= n && c.r.Fn(fn).Recv != RecvNone {
		return own[n:]
	}
	return own
}

func (c *checker) indexUsize(index TypeID) TypeID {
	tt := c.r.Types
	open := index
	if rng := c.r.langItems["Range"]; tt.Kind(index) == KNamed && rng != 0 && tt.Node(index).Ent == rng {
		open = tt.Node(index).Args[0]
	}
	if _, ok := c.vars.index(open); ok {
		c.vars.unify(open, TyUsize)
	}
	return c.vars.resolve(index)
}

func (c *checker) indexType(n, idx syntax.NodeID, recv, index TypeID) TypeID {
	tt := c.r.Types
	if recv == TyPoison || index == TyPoison {
		return TyPoison
	}
	if tt.Kind(recv) == KRef {
		recv = tt.Node(recv).Elem
	}
	c.info.Calls[n] = CallInfo{Kind: CallIndex}
	// Bounds are not proven in range, so `[]` can trap at run time.
	c.edge(EffectEdge{Kind: EdgePanic, Node: n})
	if tt.Kind(recv) == KNamed && recv != TyString && recv != TyBytes && tt.Node(recv).Ent != tt.arrayEnt && !c.r.isFixedArray(recv) {
		return c.indexWitnessType(n, idx, recv, index)
	}
	if tt.Kind(index) == KUntyped {
		index = c.adopt(c.literalNode(idx), TyUsize)
	}
	index = c.indexUsize(index)
	rng := c.r.langItems["Range"]
	isRange := rng != 0 && tt.Kind(index) == KNamed && tt.Node(index).Ent == rng && tt.Node(index).Args[0] == TyUsize
	switch {
	case recv == TyString || recv == TyBytes:
		if index == TyUsize {
			return TyU8
		}
		if isRange {
			return recv
		}
	case tt.Kind(recv) == KNamed && tt.Node(recv).Ent == tt.arrayEnt:
		if index == TyUsize {
			c.placeOf(n)
			return c.indexValueResult(n, tt.Node(recv).Args[0])
		}
		if isRange {
			return recv
		}
	case c.r.isFixedArray(recv):
		// A range would need a length the type does not carry.
		if index == TyUsize {
			c.placeOf(n)
			return c.indexValueResult(n, tt.Node(recv).Args[0])
		}
	default:
		c.errAt(n, cNotIndexable, recv)
		return TyPoison
	}
	c.errAt(idx, cIndexType, index)
	return TyPoison
}

// indexWitnessType resolves `recv[idx]` against recv's own `index` member, fixing an
// untyped index literal to its parameter type here, before any builtin usize coercion runs.
func (c *checker) indexWitnessType(n, idx syntax.NodeID, recv, index TypeID) TypeID {
	tt := c.r.Types
	set := c.r.memberSet(recv, "index")
	if set == 0 {
		c.errAt(n, cNotIndexable, recv)
		return TyPoison
	}
	m := c.r.Overloads[set].Members[0]
	sig := tt.Node(c.r.memberSig(recv, m))
	want := TyPoison
	if len(sig.Args) == 1 {
		want = sig.Args[0]
	}
	switch {
	case tt.Kind(index) == KUntyped:
		index = c.adopt(c.literalNode(idx), want)
	case want != TyPoison:
		if _, ok := c.vars.index(index); ok {
			c.vars.unify(index, want)
		}
		index = c.vars.resolve(index)
	default:
		index = c.vars.resolve(index)
	}
	if len(sig.Args) == 1 && sig.Args[0] == index {
		c.info.Calls[n] = CallInfo{Kind: CallIndexWitness, Callee: m, Plan: identityPlan([]syntax.NodeID{idx})}
		c.edge(EffectEdge{Kind: EdgeCall, Target: m, Node: n})
		return c.indexValueResult(n, sig.Elem)
	}
	c.errAt(idx, cIndexType, index)
	return TyPoison
}
