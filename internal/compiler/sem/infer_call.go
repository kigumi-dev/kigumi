package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

func (c *checker) callFn(n syntax.NodeID, fn EntityID, typeArgs []TypeID, args []syntax.NodeID, want TypeID, recv TypeID, recvBase syntax.NodeID) TypeID {
	info := c.r.Fn(fn)
	name := c.r.Entities[fn].Name
	async := info.Declared&EffAsync != 0
	if async {
		want = 0
	}
	sig, inst, badTypeArgs := c.instantiateFn(n, fn, typeArgs, recv)
	c.noteTaskUse(n, fn, recv)
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
	c.deflt.recv = recv
	c.markAtomicPlace(fn, args)
	ret, plan := c.checkArgs(n, name, sig, args, inst, want, info.Params, info.Body != 0, async)
	c.deflt.recv = 0
	c.taintRecvResult(n, ret, recvBase)
	c.crossRecv(n, fn, recv, recvBase)
	c.ffiCValueCheck(n, fn, inst, recv)
	c.ffiAddressLeaseCheck(n, fn, inst)
	c.ffiLeaseCapture(fn, args)
	c.errorAsCheck(n, fn, inst)
	c.truncateCheck(n, fn, typeArgs, inst)
	c.exitCodeCheck(fn, args)
	c.memOpCheck(n, fn, inst, args, recv)
	if async {
		// A direct call to an async fn yields a lazy Future.
		ret = c.r.Types.Named(c.r.Types.futureEnt, []TypeID{ret})
	}
	effArgs := args
	if plan != nil {
		effArgs = plan.placed()
	}
	subst := c.callSubst(fn, inst, recv)
	c.callbackEdges(n, fn, sig, effArgs, subst)
	c.argContracts(effArgs, info.Sig, subst, fn, recvBase)
	c.fieldCallbackEdges(fn, effArgs)
	c.knownFieldCallEdges(n, fn, effArgs)
	c.ifaceFieldArgEdges(n, fn, effArgs)
	c.recordBorrowFieldCallSites(n, fn, effArgs)
	c.noteContainerIntrinsic(n, fn, effArgs, recvBase, ret)
	c.noteResultSite(n, fn, effArgs, recvBase, ret)
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
	if kind := c.r.ProtoKind(fn); kind != "" && !badTypeArgs {
		c.noteProtoSite(from, n, protoSubject(recv, inst), kind, effArgs...)
	}
	c.touched = append(c.touched, n)
	return ret
}

func protoSubject(recv TypeID, inst []TypeID) TypeID {
	if recv != 0 {
		return recv
	}
	if len(inst) > 0 {
		return inst[0]
	}
	return 0
}

// declParams is nil for a bare fn value; selfVerified skips taintCallResult for a body already checked not to leak.
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
	spreadAt := slices.IndexFunc(args, func(a syntax.NodeID) bool { return c.t.Kind(a) == syntax.Spread })
	required := c.requiredParams(declParams, fixed)
	switch {
	case cVariadic && len(args) < fixed:
		c.errAt(n, cCallArity, name, fixed, plural(fixed), len(args))
		return c.abandonCall(sig, args), nil
	case !cVariadic && !variadic && (len(args) < required || len(args) > len(params)):
		c.errAt(n, cCallArity, name, arityCount(required, len(params)), plural(len(params)), len(args))
		return c.abandonCall(sig, args), nil
	case !cVariadic && variadic && len(args) < fixed:
		c.errAt(n, cCallArity, name, fixed, plural(fixed), len(args))
		return c.abandonCall(sig, args), nil
	case !cVariadic && variadic && spreadAt >= 0 && (spreadAt != len(args)-1 || len(args) != fixed+1):
		c.errAt(args[spreadAt], cVariadicMixed)
		return c.abandonCall(sig, args), nil
	}
	plan := identityPlan(args)
	if c.hasNamedArg(args) {
		place, ok := c.resolveNamedArgs(name, args, declParams, fixed)
		if !ok {
			return c.abandonCall(sig, args), nil
		}
		plan = newCallPlan(c.unwrapNamed(args), place)
	}
	if required < fixed {
		eval, place, ok := c.fillDefaults(n, name, plan.Eval, plan.Place, declParams, fixed, inst)
		if !ok {
			return c.abandonCall(sig, args), nil
		}
		plan = newCallPlan(eval, place)
	}
	if p := c.piped[n]; p != 0 {
		plan = plan.pipedFirst(p)
	}
	args = plan.placed()
	if cVariadic {
		return c.checkCArgs(n, name, sn, args), plan
	}
	for pass := 0; pass < 2; pass++ {
		c.wantBeforeLambdas(pass, sn.Elem, want, args)
		for i, a := range args {
			if c.lambdaArg(a) != (pass == 1) {
				continue
			}
			if c.t.Kind(a) == syntax.Spread {
				if !variadic {
					c.errAt(a, cSpreadNonVariadic, name)
					c.synth(c.t.View(a).AsSpread().Value().ID())
					continue
				}
				inner := c.t.View(a).AsSpread().Value().ID()
				c.checkArg(inner, params[fixed], entAt(declParams, fixed), async, callResult{})
				c.crossArg(inner, entAt(declParams, fixed))
				c.checkEscapeArg(inner, entAt(declParams, fixed), params[fixed], tt.escMarked(sn.Var, fixed), name)
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
				c.markLaterArgs(a, plan.Eval)
			}
			c.noteLentArg(a, entAt(declParams, pi), param, tt.escMarked(sn.Var, pi))
			from := len(*c.edges)
			c.markReborrowArg(a, param, entAt(declParams, pi), async)
			c.checkArg(a, param, entAt(declParams, pi), async, callResult{sn.Elem, want})
			c.rejectScalarCopyArg(a, param)
			c.crossArg(a, entAt(declParams, pi))
			c.boxedArg(a, entAt(declParams, pi))
			c.retagLendCross(from, a, entAt(declParams, pi), async)
			c.noteLentRefArg(a, entAt(declParams, pi), async)
			if pass == 0 {
				c.checkEscapeArg(a, entAt(declParams, pi), param, tt.escMarked(sn.Var, pi), name)
			}
		}
	}
	c.checkMutAliasArgs(args)
	c.rejectTwiceLent(args)
	c.noteMutBorrowSites(args, declParams)
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
