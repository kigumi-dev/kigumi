package sem

import "kigumi/internal/compiler/syntax"

// paramEnt is the declared parameter entity for the alias hint, or 0 when there is none.
// async marks a call that build_call.go's placeArg never boxes an argument for.
func (c *checker) checkArg(a syntax.NodeID, param TypeID, paramEnt EntityID, async bool, res callResult) {
	if c.t.Kind(a) == syntax.BorrowExpr {
		if async {
			if c.argAsyncPlace == nil {
				c.argAsyncPlace = map[syntax.NodeID]bool{}
			}
			c.argAsyncPlace[a] = true
		} else {
			if c.argPlace == nil {
				c.argPlace = map[syntax.NodeID]bool{}
			}
			c.argPlace[a] = true
		}
	}
	c.noteMutArg(a, paramEnt)
	defer c.rejectBranchFieldArg(a)
	defer c.reserveDrainArg(a, paramEnt)
	// After the argument is synthesized: refBinding reads info.Uses.
	defer c.noteRefArg(a, paramEnt, async)
	c.noteRefPlaceArg(a, paramEnt, async)
	aliasFile, aliasNode := c.r.paramAliasSite(paramEnt)
	c.expectErasedCall(a, param, res)
	param = c.vars.resolve(param)
	if !c.r.Types.ContainsVar(param) || c.lambdaArg(a) {
		c.checkAlias(a, param, aliasFile, aliasNode)
		if c.info.Types[a] == TyPoison {
			c.vars.poison(param)
		} else {
			c.consumeArg(a, param)
		}
		return
	}
	hint := c.vars.fnHint(param)
	var got TypeID
	if c.t.Kind(a) == syntax.BorrowExpr {
		got = c.vars.resolve(c.setType(a, c.synthBorrow(a, param)))
	} else {
		got = c.vars.resolve(c.synth(a))
		c.hintMethodValue(got, param)
	}
	if got == TyPoison || c.borrowsPoison(got) {
		c.vars.poison(param)
		c.vars.poison(hint)
		return
	}
	if ft := c.valueBoundWant(param, got); ft != 0 {
		param = ft
	} else if ft := c.erasedItemWant(a, got, param, res); ft != 0 {
		param = ft
	}
	steps, ok := c.coerceSteps(a, got, param)
	if !ok {
		c.mismatchAlias(a, param, got, aliasFile, aliasNode)
		return
	}
	if len(steps) > 0 {
		c.info.Coerce[a] = Coercion{From: got, Steps: steps}
		c.touched = append(c.touched, a)
		c.chargeCoercion(a, c.info.Coerce[a])
	}
	c.fixHintedSignature(a, got, hint)
	c.consumeArg(a, param)
}

// An fn or closure value passed for a type parameter bounded by Fn/FnMut/FnOnce fixes the type variables of the bound's signature now, so later statements see them.
func (c *checker) fixHintedSignature(a syntax.NodeID, got, hint TypeID) {
	if hint == 0 {
		return
	}
	switch c.r.Types.Kind(got) {
	case KFn:
		if item, ok := c.genericFnItemCallable(c.t.Unparen(a)); ok {
			c.coerceGenericFnItem(c.t.Unparen(a), item, got, hint)
			return
		}
		if c.sameFnShape(got, hint) {
			c.refScopeContract(a, got, hint)
		}
	case KClosure:
		if c.sameFnShape(c.r.closure(c.r.Types.Node(got).Ent).Sig, hint) {
			c.refScopeContract(a, got, hint)
		}
	case KParam:
		for _, s := range c.paramShapes(got) {
			if c.sameFnShape(s, hint) {
				return
			}
		}
	}
}

// consumeArg marks a by-value argument consumed; a &T param borrows instead of moving.
func (c *checker) consumeArg(a syntax.NodeID, param TypeID) {
	if c.r.Types.Kind(param) == KRef {
		return
	}
	c.noteOnceUse(a, a, c.info.Types[a])
	c.consumeWhole(a)
}

// entAt is params[i], or 0 when i is out of range (e.g. no source declaration).
func entAt(params []EntityID, i int) EntityID {
	if i < 0 || i >= len(params) {
		return 0
	}
	return params[i]
}

func (c *checker) unifyWant(n syntax.NodeID, ret, want TypeID) {
	tt := c.r.Types
	var steps []CoStep
	for {
		if c.vars.unify(ret, want) {
			break
		}
		if elem, ok := tt.IsOption(want); ok {
			steps = append([]CoStep{{CoSome, want}}, steps...)
			want = elem
			continue
		}
		if val, _, ok := tt.IsResult(want); ok {
			steps = append([]CoStep{{CoOk, want}}, steps...)
			want = val
			continue
		}
		return
	}
	if len(steps) > 0 {
		c.info.Coerce[n] = Coercion{From: ret, Steps: steps}
		c.chargeCoercion(n, c.info.Coerce[n])
	}
}

func (c *checker) borrowsPoison(t TypeID) bool {
	tt := c.r.Types
	return tt.Kind(t) == KRef && tt.Node(t).Elem == TyPoison
}
