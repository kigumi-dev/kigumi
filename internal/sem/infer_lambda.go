package sem

import "kigumi/internal/syntax"

// lambdaCtx: one lambda body under check.
type lambdaCtx struct {
	ent      EntityID
	scope    ScopeID
	captured map[EntityID]int
	list     []Capture
	// refCapture: an E412 (borrow captured) already explains a borrow-typed body.
	refCapture bool
	immediate  bool
	// reserveUses are uses of cleanup-reserved bindings, judged when the lambda closes.
	reserveUses []captureUse
	writes      []EntityID
	consumes    []lambdaConsume
	// keptMut are captures of a local another closure writes, rejected unless they turn out only read.
	keptMut map[EntityID]syntax.NodeID
}

// synthLambda: want is a fn type, an Fn-constrained type parameter, or 0
// (then every parameter needs an annotation).
func (c *checker) synthLambda(n syntax.NodeID, want TypeID) TypeID {
	lam := c.t.View(n).AsLambda()
	params := lam.Params().AppendIDs(nil)
	body := lam.Body().ID()
	tt := c.r.Types
	target := c.lambdaTarget(want)
	if target != 0 && len(tt.Node(target).Args) != len(params) {
		c.errAt(n, cLambdaArity, len(params), plural(len(params)), target, len(tt.Node(target).Args))
		return TyPoison
	}
	id := c.r.newEntity(Entity{Kind: EntClosure, Name: "closure", Pkg: c.pkg, File: c.f, Node: n, Tok: lam.Tok().Index(), Parent: c.fn})
	c.r.Entities[id].Detail = c.r.addClosure(ClosureInfo{})
	c.info.Defs[n] = id
	saved := c.scope
	c.pushScope(ScopeLambda, n)
	ctx := &lambdaCtx{ent: id, scope: c.scope, captured: map[EntityID]int{}, immediate: c.argLambda[n]}
	c.lambdas = append(c.lambdas, ctx)
	ptypes, pents, bad := c.lambdaParams(id, params, target)
	ret := TypeID(0)
	if target != 0 {
		ret = tt.Node(target).Elem
	}
	savedEdges := c.edges
	var edges []EffectEdge
	c.edges = &edges
	savedLocals := c.locals
	c.locals = nil
	savedConsumed, savedBranch := c.consumed, c.branch
	c.consumed, c.branch = nil, 0
	ret = c.lambdaBody(body, ret)
	// A closure is reached through a value, so its result crosses as a carrier; a payload-free variant's box is an immortal.
	if res := c.vars.resolve(ret); c.r.Types.ContainsVar(res) || c.r.containsClosure(res) || c.r.BoxAllocates(res) {
		c.crossDirect(body, res)
	}
	if !c.constScalar(body) && !c.readsBox(body) {
		c.scalarEdge(n, ret, whyClosureScalar)
	}
	c.dropsAtScopeExit(pents)
	c.edges = savedEdges
	c.locals = savedLocals
	c.consumed, c.branch = savedConsumed, savedBranch
	c.lambdas = c.lambdas[:len(c.lambdas)-1]
	c.settleCaptureReserves(ctx)
	c.popScope(saved)
	if bad {
		return TyPoison
	}
	info := c.r.closure(id)
	info.Params = pents
	info.Ret = ret
	info.Sig = tt.WithEscParams(tt.Fn(ptypes, ret, 0, c.lambdaVariadic(params, target)), c.r.escapingMarks(ptypes, c.escapingFlags(pents)))
	c.demoteReadCaptures(ctx)
	c.rejectKeptMut(ctx)
	info.Captures = ctx.list
	info.Copy = closureCopy(ctx.list)
	info.Once = c.lambdaOnce(ctx)
	c.noteNestedCaptureMoves(n, ctx)
	info.Immediate = ctx.immediate
	info.Edges = edges
	c.closures = append(c.closures, id)
	result := tt.Closure(id)
	info.Creator = c.fn
	if depth := len(c.lambdas); depth > 0 {
		info.Creator = c.lambdas[depth-1].ent
	}
	info.Scoped = c.allocDepth > 0
	// With nothing captured the environment is empty and needs no storage.
	if len(info.Captures) > 0 {
		c.buildEdge(n, result)
	}
	return result
}

func (c *checker) lambdaTarget(want TypeID) TypeID {
	want = c.vars.resolve(want)
	if want == 0 {
		return 0
	}
	tt := c.r.Types
	switch tt.Kind(want) {
	case KFn:
		return want
	case KVar:
		return c.vars.fnHint(want)
	case KParam:
		for _, con := range c.r.typeParam(tt.Node(want).Ent).Constraints {
			if con.Kind == CFn || con.Kind == CFnMut || con.Kind == CFnOnce {
				return con.Type
			}
		}
	}
	return 0
}

func (c *checker) lambdaBody(body syntax.NodeID, ret TypeID) TypeID {
	savedRet, savedLoop, savedCleanup, savedStmt := c.retType, c.loop, c.cleanup, c.stmtPos
	savedBreaks := c.breaks
	savedFirst := c.vars.first
	entryFacts := c.snapshotFacts()
	c.loop, c.cleanup, c.breaks = 0, 0, nil
	tt := c.r.Types
	own := ret == 0
	if own {
		ret = c.vars.fresh(body, "return")
	}
	c.retType = ret
	c.vars.first = c.vars.count()
	if !tt.ContainsVar(c.vars.resolve(ret)) {
		c.check(body, ret)
	} else {
		got := c.vars.resolve(c.synth(body))
		if tt.Kind(got) == KUntyped {
			got = c.adopt(c.literalNode(body), defaultOf(got))
		}
		if got == TyPoison {
			c.vars.poison(ret)
		} else if got != TyNever && !c.vars.unify(ret, got) {
			c.mismatch(body, ret, got)
		} else if got == TyNever && own && tt.Kind(c.vars.resolve(ret)) == KVar {
			c.vars.unify(ret, TyNever)
		}
	}
	c.checkEscapeTailReturn(body)
	c.noteLambdaTail(body)
	c.retType, c.loop, c.cleanup, c.stmtPos = savedRet, savedLoop, savedCleanup, savedStmt
	c.breaks = savedBreaks
	c.vars.first = savedFirst
	c.facts = entryFacts
	out := c.vars.resolve(ret)
	if tt.Kind(out) == KRef && !c.lambdas[len(c.lambdas)-1].refCapture {
		c.errAt(body, cBorrowPosition, "return types")
		return TyPoison
	}
	return out
}

func (c *checker) lambdaRetUnknown() bool {
	return len(c.lambdas) > 0 && c.r.Types.Kind(c.vars.resolve(c.retType)) == KVar
}
