package sem

import "kigumi/internal/syntax"

func (c *checker) synthArgs(args []syntax.NodeID) {
	for _, a := range args {
		if c.t.Kind(a) == syntax.Spread || c.t.Kind(a) == syntax.NamedArg {
			c.synth(syntax.NodeID(c.t.Nodes[a].Lhs))
			continue
		}
		c.synth(a)
	}
}

func (c *checker) callFn(n syntax.NodeID, fn EntityID, typeArgs []TypeID, args []syntax.NodeID, want TypeID, recv TypeID, recvBase syntax.NodeID) TypeID {
	info := c.r.Fn(fn)
	name := c.r.Entities[fn].Name
	async := info.Declared&EffAsync != 0
	if async {
		want = 0
	}
	sig, inst, badTypeArgs := c.instantiateFn(n, fn, typeArgs, recv)
	from := len(*c.edges)
	c.edge(EffectEdge{Kind: EdgeCall, Target: fn, Node: n})
	c.callInvalidates()
	if (info.Declared&EffUnsafe != 0 || info.Abi != "" || info.Naked) && c.unsafe == 0 {
		c.errAt(n, cUnsafeRequired, "calling `"+name+"`")
	}
	if ret, done := c.dlCall(n, fn, sig, inst, recv, args, want); done {
		return ret
	}
	kind := CallFn
	if info.Recv != RecvNone {
		kind = CallMethod
	} else if info.Owner != 0 {
		kind = CallAssoc
	}
	ret, plan := c.checkArgs(n, name, sig, args, inst, want, info.Params, info.Body != 0, async)
	c.crossRecv(n, fn, recv, recvBase)
	c.crossAsyncResult(n, ret)
	c.ffiCValueCheck(n, fn, inst, recv)
	c.ffiAddressLeaseCheck(n, fn, inst)
	c.errorAsCheck(n, fn, inst)
	if async {
		// A direct call to an async fn yields a lazy Future.
		ret = c.r.Types.Named(c.r.Types.futureEnt, []TypeID{ret})
	}
	effArgs := args
	if plan != nil {
		effArgs = plan.placed()
	}
	c.callbackEdges(n, fn, sig, effArgs)
	c.fieldCallbackEdges(fn, effArgs)
	c.knownFieldCallEdges(n, fn, effArgs)
	c.ifaceFieldArgEdges(n, fn, effArgs)
	c.recordBorrowFieldCallSites(n, fn, effArgs)
	if info.Recv == RecvMut && recvBase != 0 {
		c.recordMethodFieldCallSite(n, fn, recvBase, effArgs)
	}
	call := CallInfo{Kind: kind, Callee: fn, Inst: inst, Recv: info.Recv, Variadic: c.variadicForm(effArgs, sig), Plan: plan}
	c.info.Calls[n] = call
	// badTypeArgs already reported cTypeArgCount; inst is made-up, so skip
	// to avoid a second diagnostic for the same mistake.
	if !badTypeArgs {
		c.deferWitnesses(n, fn, recv, inst)
	}
	c.noteInline(n, recvBase, fn, args, from)
	c.touched = append(c.touched, n)
	return ret
}

// declParams may be nil (no declaration backs sig, e.g. a bare fn value).
// selfVerified: the callee's own body was already checked not to leak its arguments, so taintCallResult is skipped.
// The second result is the call's plan, or nil on an arity/named-argument error.
func (c *checker) checkArgs(n syntax.NodeID, name string, sig TypeID, args []syntax.NodeID, inst []TypeID, want TypeID, declParams []EntityID, selfVerified, async bool) (TypeID, *CallPlan) {
	tt := c.r.Types
	sn := tt.Node(sig)
	params := sn.Args
	cVariadic := sn.Flags&fnCVariadic != 0
	variadic := sn.Flags&fnVariadic != 0
	fixed := len(params)
	if variadic {
		fixed--
	}
	spread := len(args) > 0 && c.t.Kind(args[len(args)-1]) == syntax.Spread
	switch {
	case cVariadic && len(args) < fixed:
		c.errAt(n, cCallArity, name, fixed, plural(fixed), len(args))
		return c.abandonCall(sig, args), nil
	case !cVariadic && !variadic && len(args) != len(params):
		c.errAt(n, cCallArity, name, len(params), plural(len(params)), len(args))
		return c.abandonCall(sig, args), nil
	case !cVariadic && variadic && len(args) < fixed:
		c.errAt(n, cCallArity, name, fixed, plural(fixed), len(args))
		return c.abandonCall(sig, args), nil
	case !cVariadic && variadic && spread && len(args) != fixed+1:
		c.errAt(args[len(args)-1], cVariadicMixed)
		return c.abandonCall(sig, args), nil
	}
	plan := identityPlan(args)
	if c.hasNamedArg(args) {
		place, ok := c.resolveNamedArgs(name, args, declParams, fixed)
		if !ok {
			return c.abandonCall(sig, args), nil
		}
		plan = newCallPlan(c.unwrapNamed(args), place)
		args = plan.placed()
	}
	if cVariadic {
		return c.checkCArgs(n, name, sn, args), plan
	}
	for pass := 0; pass < 2; pass++ {
		for i, a := range args {
			if (c.t.Kind(a) == syntax.Lambda) != (pass == 1) {
				continue
			}
			if c.t.Kind(a) == syntax.Spread {
				if !variadic {
					c.errAt(a, cSpreadNonVariadic, name)
					c.synth(syntax.NodeID(c.t.Nodes[a].Lhs))
					continue
				}
				inner := syntax.NodeID(c.t.Nodes[a].Lhs)
				c.checkArg(inner, params[fixed], entAt(declParams, fixed), async)
				c.crossArg(inner, entAt(declParams, fixed))
				continue
			}
			pi := min(i, len(params)-1)
			param := params[pi]
			if variadic && i >= fixed {
				param = tt.Node(params[fixed]).Args[0]
				pi = fixed
			}
			if pass == 1 {
				c.markImmediate(a, c.r.nonEscapingParam(entAt(declParams, pi)))
			}
			c.checkArg(a, param, entAt(declParams, pi), async)
			c.crossArg(a, entAt(declParams, pi))
			c.noteLentRefArg(a, entAt(declParams, pi), async)
			if pass == 0 {
				c.checkEscapeArg(a, entAt(declParams, pi), param, name)
			}
		}
	}
	c.checkMutAliasArgs(args)
	tailSpread := len(args) > 0 && c.t.Kind(args[len(args)-1]) == syntax.Spread
	if variadic && !tailSpread {
		c.edge(EffectEdge{Kind: EdgeAlloc, Node: n, Why: 2})
	}
	ret := sn.Elem
	if want != 0 && tt.ContainsVar(c.vars.resolve(ret)) {
		c.unifyWant(n, ret, want)
	}
	rt := c.vars.resolve(ret)
	if !selfVerified {
		c.taintCallResult(n, rt, args)
	}
	return rt, plan
}
