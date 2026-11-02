package sem

import "kigumi/internal/compiler/syntax"

// synthBracket implements `a[b]`.
func (c *checker) synthBracket(n syntax.NodeID) TypeID {
	br := c.t.View(n).AsBracket()
	base := br.Base().ID()
	items := br.Args()
	head := c.resolveHead(base)
	switch head.kind {
	case headFn:
		if params := c.ownGenerics(head.ent); len(params) > 0 {
			return c.explicitFnItem(n, head.ent, params, items)
		}
	case headType:
		if len(c.r.typeDecl(head.ent).Params) > 0 {
			c.errAt(n, cTypeAsValue, c.r.Entities[head.ent].Name)
			return TyPoison
		}
	case headSet:
		c.errAt(n, cOverloadValueAmbig, c.r.Overloads[head.set].Name)
		return TyPoison
	case headVariant:
		return c.variantTypeArgs(n, head.ent, items)
	case headNone:
		return TyPoison
	}
	recv := c.vars.resolve(c.synthPlace(base))
	if c.isMethodRef(base) {
		return c.methodRefTypeArgs(n, base, items, recv)
	}
	if items.Len() != 1 {
		c.errAt(n, cIndexCount)
		return TyPoison
	}
	c.rejectJoinBase(base)
	index := c.vars.resolve(c.synth(items.ItemID(0)))
	return c.indexType(n, items.ItemID(0), recv, index)
}

// indexValueResult gates a by-value index read on T: Copy. n is exempt when it was
// reached as a place (synthPlace) to extend further, not to materialize a value.
func (c *checker) indexValueResult(n syntax.NodeID, elem TypeID) TypeID {
	if c.indexAsPlace[n] {
		delete(c.indexAsPlace, n)
		return elem
	}
	if !c.copyNow(elem) {
		c.errAtHelp(n, c.indexValueHelp(n, elem), cIndexValueCopy, elem)
	}
	return elem
}

// checkIndexFieldValue extends indexValueResult to a field read whose base is
// (transitively) an index: `xs[i].field` is as untracked as `xs[i]` itself.
func (c *checker) checkIndexFieldValue(n syntax.NodeID, t TypeID) TypeID {
	if t == 0 || t == TyPoison || !c.indexRooted(n) || c.copyNow(t) {
		return t
	}
	c.errAtHelp(n, c.indexValueHelp(n, t), cIndexValueCopy, t)
	return t
}

// indexRooted reports whether n bottoms out, through parens and field projections,
// at a bracket index. Walks the syntax directly so an unresolvable base can't hide it.
func (c *checker) indexRooted(n syntax.NodeID) bool {
	_, ok := c.indexRoot(n)
	return ok
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
	c.errAtHelp(idx, usizeHelp(TyUsize, index), cIndexType, index)
	return TyPoison
}

// An open numeric literal variable settles to a scalar.
func (c *checker) copyNow(t TypeID) bool {
	t = c.vars.resolve(t)
	return c.vars.pendingLiteral(t) || c.r.isCopy(t)
}
