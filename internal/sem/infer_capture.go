package sem

import "kigumi/internal/syntax"

// captureSite keeps ctx and idx valid even after ctx.list is copied into
// ClosureInfo.Captures, so a write discovered later can still find the entry.
type captureSite struct {
	ctx *lambdaCtx
	idx int
}

func (c *checker) noteCapture(n syntax.NodeID, ent EntityID, found ScopeID) {
	for i := len(c.lambdas) - 1; i >= 0; i-- {
		ctx := c.lambdas[i]
		if c.scopeWithin(found, ctx.scope) {
			return
		}
		if _, ok := ctx.captured[ent]; ok {
			c.deferCaptureReserve(ctx, n, ent)
			continue
		}
		e := &c.r.Entities[ent]
		mode := capMove
		if c.r.isCopy(e.Type) {
			mode = capCopy
		}
		switch {
		case ctx.immediate:
			if mode == capMove {
				mode = capBorrow
			}
		// A plain `self` has no write-back cell, so it is lent like a `&T` param.
		case c.r.Types.Kind(e.Type) == KRef, mode == capMove && plainSelfReceiver(e):
			c.errAt(n, cClosureCapturesRef, e.Name)
			ctx.refCapture = true
		case c.r.typeHasLifetime(e.Type):
			// A non-immediate closure can escape its call, so a captured
			// borrow-carrying value would outlive its owner.
			c.errAt(n, cLifetimeEscapes, c.r.TypeString(e.Type), "a closure")
		case c.restrictedEntity(ent):
			c.errAt(n, cEscapingCaptured, c.r.Entities[c.aliasChase(ent)].Name, c.r.escapingForm(c.aliasChase(ent)))
		}
		if mode == capMove {
			c.deferCaptureReserve(ctx, n, ent)
		}
		// Start shared rather than waiting for a write this checker
		// won't see again once ctx is off c.lambdas.
		if mode == capCopy && e.Flags&EfCapturedMut != 0 {
			mode = capCell
		}
		// A `mut self` receiver is the caller's write-back cell, so a closure over it shares that cell.
		if lentReceiver(e) {
			mode = capCell
		}
		if mode == capMove && e.Flags&EfCapturedMut != 0 {
			if ctx.keptMut == nil {
				ctx.keptMut = map[EntityID]syntax.NodeID{}
			}
			ctx.keptMut[ent] = n
		}
		ctx.captured[ent] = len(ctx.list)
		ctx.list = append(ctx.list, Capture{Local: ent, Mode: mode, OwnCell: mode == capMove && c.ownCellAbove(i, ent)})
		c.captureSites[ent] = append(c.captureSites[ent], captureSite{ctx: ctx, idx: len(ctx.list) - 1})
	}
}

func (c *checker) capturedHere(ent EntityID) bool {
	if len(c.lambdas) == 0 {
		return false
	}
	_, ok := c.lambdas[len(c.lambdas)-1].captured[ent]
	return ok
}

func (c *checker) noteCaptureWrite(n syntax.NodeID, ent EntityID) {
	// A write through a borrow changes the referent, not the local.
	if c.r.Types.Kind(c.r.Entities[ent].Type) == KRef {
		return
	}
	c.noteLambdaWrite(ent)
	if c.capturedHere(ent) {
		c.edge(EffectEdge{Kind: EdgeMutateCaller, Target: ent, Node: n})
	}
	sites := c.captureSites[ent]
	if len(sites) == 0 || c.movedCaptureWrite(n, ent, sites) {
		return
	}
	c.r.Entities[ent].Flags |= EfCapturedMut
	c.noteMutCapture(n, ent)
	// Every closure that captured ent needs a shared cell, even ones already
	// closed: ctx.list aliases the backing array copied into ClosureInfo.
	for _, site := range sites {
		if site.ctx.list[site.idx].Mode == capCopy {
			site.ctx.list[site.idx].Mode = capCell
		}
	}
}

// markImmediate records a lambda passed to a verified non-escaping parameter; only then may its
// captures borrow instead of move, since the callee's contract guarantees it won't outlive the call.
func (c *checker) markImmediate(lambda syntax.NodeID, nonEscaping bool) {
	if !nonEscaping {
		return
	}
	if c.argLambda == nil {
		c.argLambda = map[syntax.NodeID]bool{}
	}
	c.argLambda[lambda] = true
}

func (c *checker) scopeWithin(s, root ScopeID) bool {
	for ; s != 0; s = c.r.Scopes[s].Parent {
		if s == root {
			return true
		}
	}
	return false
}

func closureCopy(caps []Capture) bool {
	for _, cap := range caps {
		if cap.Mode == capMove || cap.Mode == capRetain || cap.Mode == capBorrow || cap.Mode == capRead {
			return false
		}
	}
	return true
}

// closureErase coerces a closure to a plain fn type; cell and move-only captures cannot erase.
func (c *checker) closureErase(n syntax.NodeID, closure EntityID, want TypeID) ([]CoStep, bool) {
	info := c.r.closure(closure)
	if !c.sameFnShape(info.Sig, want) {
		return nil, false
	}
	if i, ok := c.firstEscapeMismatch(info.Sig, want); ok {
		c.errAt(n, cEscapingCoerce, c.typeText(info.Sig), i+1, c.typeText(want))
		return nil, true
	}
	for _, cap := range info.Captures {
		name := c.capturedName(n, info, cap.Local)
		switch cap.Mode {
		case capCell:
			c.errAtHelp(n, c.returnedClosureHelp(want, false), cClosureNotPlainFn, name, c.r.cellCaptureWhy(cap.Local))
			return nil, true
		case capMove, capRetain, capRead:
			c.errAtHelp(n, c.returnedClosureHelp(want, true), cClosureNotPlainFn, name, "move-only")
			return nil, true
		}
	}
	need := Effects(c.r.Types.Node(want).Flags) & (EffPure | EffNoalloc)
	if need != 0 {
		c.edge(EffectEdge{Kind: EdgeContract, Target: closure, Mods: need, Node: n})
	}
	if len(info.Captures) > 0 {
		c.edge(EffectEdge{Kind: EdgeAlloc, Type: want, From: c.r.Types.Closure(closure), Node: n})
	}
	return []CoStep{{CoClosureErase, want}}, true
}

// sameFnShape judges two fn signatures interchangeable for closure erasure and Fn/FnMut/
// FnOnce satisfaction, ignoring escaping-vs-plain mismatches: enforced separately at call sites.
func (c *checker) sameFnShape(got, want TypeID) bool {
	tt := c.r.Types
	g, w := tt.Node(c.vars.resolve(got)), tt.Node(c.vars.resolve(want))
	if len(g.Args) != len(w.Args) || g.Flags&fnVariadic != w.Flags&fnVariadic {
		return false
	}
	mark := c.vars.mark()
	for i := range g.Args {
		if !c.vars.unifyIgnoringEscaping(g.Args[i], w.Args[i]) {
			c.vars.rollback(mark)
			return false
		}
	}
	if !c.vars.unifyIgnoringEscaping(g.Elem, w.Elem) {
		c.vars.rollback(mark)
		return false
	}
	return true
}

func lentReceiver(e *Entity) bool {
	const want = EfSelf | EfMut
	return e.Flags&want == want && e.Flags&EfMove == 0
}

func plainSelfReceiver(e *Entity) bool {
	return e.Flags&(EfSelf|EfMut|EfMove) == EfSelf
}

func (r *Result) cellCaptureWhy(ent EntityID) string {
	if lentReceiver(&r.Entities[ent]) {
		return "a receiver lent by the caller"
	}
	return "assigned inside the closure"
}
