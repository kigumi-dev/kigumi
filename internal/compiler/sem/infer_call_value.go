package sem

import "kigumi/internal/compiler/syntax"

func (c *checker) callValue(n, callee syntax.NodeID, ft TypeID, args []syntax.NodeID, want TypeID) TypeID {
	tt := c.r.Types
	if ft == TyPoison {
		c.synthArgs(args)
		return TyPoison
	}
	c.noteOnceUse(n, callee, ft)
	if tt.Kind(ft) == KRef {
		var ok bool
		if ft, ok = c.borrowedCallee(n, ft); !ok {
			c.synthArgs(args)
			return TyPoison
		}
	}
	node := tt.Node(ft)
	why := uint8(whyDynamicResult)
	var closure EntityID
	if node.Kind == KClosure {
		// A method value runs the method through the same adapter as a plain fn value.
		if !c.r.closure(node.Ent).IsMethodValue() {
			closure = node.Ent
		}
		ft = c.vars.resolve(c.r.closure(node.Ent).Sig)
		node = tt.Node(ft)
	}
	if node.Kind == KParam {
		why = whyWitnessResult
		for _, con := range c.r.typeParam(node.Ent).Constraints {
			if con.Kind == CFn || con.Kind == CFnMut || con.Kind == CFnOnce {
				ft = con.Type
				node = tt.Node(ft)
				break
			}
		}
	}
	if node.Kind != KFn {
		c.synthArgs(args)
		c.errAt(n, cNotCallable, ft)
		return TyPoison
	}
	c.callInvalidates()
	if closure != 0 && c.r.closure(closure).Once != "" {
		if p, ok := c.consumedPlace(callee); ok {
			c.noteLambdaConsume(callee, p)
		}
	}
	if fact, ok := c.fieldFact[n]; ok {
		delete(c.fieldFact, n)
		c.applyFieldFact(n, fact)
	} else if closure != 0 {
		c.edge(EffectEdge{Kind: EdgeCall, Target: closure, Node: n})
		c.noteClosureResult(n, closure)
	} else {
		c.callbackEdge(n, n, Effects(node.Flags), 0, false)
		key, viaField := c.knownFieldCallKeyOf(callee)
		for _, tgt := range c.knownCallTargets(callee) {
			if viaField && c.r.fnKilledAt(key, tgt, c.f, n) {
				continue
			}
			c.edge(EffectEdge{Kind: EdgeKnownCall, Target: tgt, Node: n})
			if viaField {
				c.r.noteEdgeSite(key, c.f, n, tgt, 0)
			}
		}
		if callee != 0 {
			if key, ok := c.fnFieldSrcKey(callee); ok {
				c.stageLateFieldEdge(EdgeKnownCall, key, 0, n, c.fnFieldReadNode(callee))
			}
		}
	}
	if node.Flags&uint16(EffUnsafe) != 0 && c.unsafe == 0 {
		c.errAt(n, cUnsafeRequired, "calling an `unsafe fn` value")
	}
	if node.Flags&fnCAbi != 0 && c.unsafe == 0 {
		c.errAt(n, cUnsafeRequired, "calling a C function pointer")
	}
	ret, plan := c.checkArgs(n, "this function", ft, args, nil, want, nil, false, false)
	if closure == 0 {
		c.boxedResult(n, ret, why)
	}
	effArgs := args
	if plan != nil {
		effArgs = plan.placed()
	}
	switch {
	case closure != 0:
		c.crossScalarArgs(effArgs, whyClosureScalar)
	case node.Flags&fnCAbi == 0:
		c.crossScalarArgs(effArgs, whyScalarCross)
	}
	c.info.Calls[n] = CallInfo{Kind: CallFnValue, Variadic: c.variadicForm(effArgs, ft), Plan: plan}
	for _, tgt := range c.knownCallTargets(callee) {
		if c.r.Entities[tgt].Kind == EntFn {
			c.recordBorrowFieldCallSites(n, tgt, effArgs)
		}
	}
	return ret
}
