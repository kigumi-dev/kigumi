package sem

import "kigumi/internal/compiler/syntax"

// paramEnt is the declared parameter entity for the alias hint, or 0 when there is none.
// async marks a call that build_call.go's placeArg never boxes an argument for.
func (c *checker) checkArg(a syntax.NodeID, param TypeID, paramEnt EntityID, async bool, res callResult) {
	c.checkArgAt(a, param, paramEnt, async, res, true)
}

// checkFieldArg checks a record literal's field value: it is stored in the new
// aggregate, not written back through a call argument, so a scalar `&mut`
// field/element inside it must be rejected instead of marked as an arg place.
func (c *checker) checkFieldArg(a syntax.NodeID, param TypeID) {
	c.checkArgAt(a, param, 0, false, callResult{}, false)
}

func (c *checker) checkArgAt(a syntax.NodeID, param TypeID, paramEnt EntityID, async bool, res callResult, callArg bool) {
	if callArg {
		c.rejectBorrowToGenericValue(a, paramEnt)
	}
	if callArg && c.t.Kind(a) == syntax.BorrowExpr {
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

// A borrow passed for a by-value type parameter has no cell the callee can
// write back through; the callee owns and drops the reference instead. The
// scalar field/element form used to slip past the MIR owned-slot check.
func (c *checker) rejectBorrowToGenericValue(a syntax.NodeID, paramEnt EntityID) {
	if paramEnt == 0 || c.t.Kind(a) != syntax.BorrowExpr {
		return
	}
	if c.r.Types.Kind(c.vars.resolve(c.r.Entities[paramEnt].Type)) != KParam {
		return
	}
	if c.t.View(a).AsBorrow().Flags()&syntax.FlagMut == 0 {
		return
	}
	place := c.t.Unparen(c.t.View(a).AsBorrow().Operand().ID())
	switch c.t.Kind(place) {
	case syntax.MemberExpr, syntax.BracketExpr:
	default:
		// A bare local borrow is the MIR owned-slot check's case; this sem
		// check exists for the scalar field/element write-back lowering.
		return
	}
	e := c.r.Entities[paramEnt]
	// A callee that returns the parameter (directly or inside another type)
	// only forwards the borrow; that is a well-formed borrow transfer.
	ret := c.r.Types.Node(c.r.Fn(e.Parent).Sig).Elem
	for _, p := range c.r.Types.Params(c.vars.resolve(ret)) {
		if p == c.r.Types.Node(c.vars.resolve(e.Type)).Ent {
			return
		}
	}
	callee := c.r.Entities[e.Parent].Name
	c.errAt(a, cBorrowOwnedParam, e.Name, callee, c.r.TypeString(e.Type))
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
	if !c.rejectArgTake(a, param) {
		c.consumeWhole(a)
	}
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
