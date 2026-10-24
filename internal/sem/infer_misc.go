package sem

import "kigumi/internal/syntax"

func (c *checker) synthString(n syntax.NodeID) TypeID {
	c.warnBareInterpolation(n)
	parts := c.t.StringParts(n)
	interpolated := false
	for _, p := range parts {
		if p.Expr == 0 {
			continue
		}
		interpolated = true
		t := c.readThrough(c.vars.resolve(c.synth(p.Expr)))
		tt := c.r.Types
		switch {
		case t == TyPoison, t == TyString, t == TyChar, t == TyBool, tt.IsNumeric(t), c.vars.pendingLiteral(t):
		case tt.Kind(t) == KUntyped:
			c.adopt(c.literalNode(p.Expr), defaultOf(t))
		default:
			c.errAtHelp(p.Expr, c.displayHelp(t), cInterpolationType, t)
		}
	}
	if interpolated {
		c.edge(EffectEdge{Kind: EdgeAlloc, Node: n, Why: 1})
		c.info.Calls[n] = CallInfo{Kind: CallInterp}
	}
	return TyString
}

func (c *checker) synthShell(n syntax.NodeID) TypeID {
	plan := c.r.langItem(c.f, n, "Plan")
	c.shellInterps(c.t.View(n).AsShellLit().Plan().ID())
	if plan == 0 {
		return TyPoison
	}
	return c.r.Types.Named(plan, nil)
}

func (c *checker) shellInterps(n syntax.NodeID) {
	if n == 0 {
		return
	}
	if c.t.Kind(n) == syntax.ShellInterp {
		e := c.t.View(n).AsShellInterp().Expr().ID()
		t := c.vars.resolve(c.synth(e))
		tt := c.r.Types
		switch {
		case t == TyPoison, t == TyString, tt.IsInteger(t):
		case tt.Kind(t) == KUntyped && t == TyUntypedInt:
			c.adopt(c.literalNode(e), TyI64)
		default:
			c.errAt(e, cShellInterpType, t)
		}
		return
	}
	for ch := range c.t.View(n).Children() {
		c.shellInterps(ch.ID())
	}
}

func (c *checker) synthComptime(n syntax.NodeID) TypeID {
	inner := c.t.View(n).AsComptime().Body().ID()
	if !c.constShaped(inner) {
		return c.synthComptimeBlock(n, inner)
	}
	v, ok := c.r.evalConst(c.f, inner)
	if !ok {
		return TyPoison
	}
	if v.Kind == constInt || v.Kind == constFloat {
		c.info.Literals[n] = v
		c.info.Literals[inner] = v
	}
	return c.setType(inner, c.r.untypedOf(v))
}

// Anything else runs in the comptime sandbox.
func (c *checker) constShaped(n syntax.NodeID) bool {
	switch v := c.t.View(n); v.Kind() {
	case syntax.IntLit, syntax.FloatLit, syntax.BoolLit, syntax.CharLit, syntax.StringLit, syntax.ByteStringLit, syntax.Ident, syntax.MemberExpr:
		return true
	case syntax.Paren:
		return c.constShaped(v.AsParen().Inner().ID())
	case syntax.Unary:
		return c.constShaped(v.AsUnary().Operand().ID())
	case syntax.Binary:
		return c.constShaped(v.AsBinary().Left().ID()) && c.constShaped(v.AsBinary().Right().ID())
	}
	return false
}

func (c *checker) synthAllocator(n syntax.NodeID, want TypeID) TypeID {
	alloc := c.t.View(n).AsAllocator()
	scopeType := c.vars.resolve(c.synth(alloc.Scope().ID()))
	if item := c.r.langItem(c.f, n, "AllocatorScope"); item != 0 && scopeType != TyPoison {
		if scopeType != c.r.Types.Named(item, nil) {
			c.errAt(alloc.Scope().ID(), cAllocatorScopeType, scopeType)
		}
	}
	c.finishExpr()
	c.r.allocBlocks = append(c.r.allocBlocks, allocBlock{file: c.f, body: alloc.Body().ID()})
	c.allocDepth++
	t := c.check(alloc.Body().ID(), want)
	c.allocDepth--
	return t
}

// finishExpr marks a sub-expression (condition, iterable, scrutinee) as its own full expression.
func (c *checker) finishExpr() {}

// The effects pass reads the EntContract entity created here.
func (c *checker) synthContract(n syntax.NodeID, want TypeID) TypeID {
	contract := c.t.View(n).AsContract()
	id := c.r.newEntity(Entity{Kind: EntContract, Name: "contract", Pkg: c.pkg, File: c.f, Node: n, Parent: c.fn})
	c.r.Entities[id].Detail = c.r.addFn(FnInfo{Declared: Effects(contract.Mods()), Body: contract.Body().ID()})
	c.info.Defs[n] = id
	saved := c.edges
	var edges []EffectEdge
	c.edges = &edges
	savedLocals := c.locals
	c.locals = nil
	savedConsumed, savedBranch := c.consumed, c.branch
	c.consumed, c.branch = consumeLog{}, 0
	c.contracts = append(c.contracts, Effects(contract.Mods()))
	t := c.check(contract.Body().ID(), want)
	c.contracts = c.contracts[:len(c.contracts)-1]
	c.dropsAtScopeExit(nil)
	c.edges = saved
	c.locals = savedLocals
	c.consumed, c.branch = savedConsumed, savedBranch
	c.r.Fn(id).Edges = edges
	c.edge(EffectEdge{Kind: EdgeCall, Target: id, Node: n})
	return t
}

func (c *checker) synthOptMember(n syntax.NodeID) TypeID {
	member := c.t.View(n).AsOptMember()
	base := c.vars.resolve(c.synthPlace(member.Base().ID()))
	if bn := c.r.Types.Node(base); bn.Kind == KRef {
		if _, isOpt := c.r.Types.IsOption(c.vars.resolve(bn.Elem)); isOpt {
			return c.optChainOn(n, n, base)
		}
	}
	elem, ok := c.r.Types.IsOption(base)
	if !ok {
		if base != TyPoison {
			c.errAt(n, cOptMemberNotOption, base)
		}
		return TyPoison
	}
	t := c.memberOn(n, member.Base().ID(), elem, member.Member().Text())
	if t == TyPoison {
		return TyPoison
	}
	if c.isPlace(member.Base().ID()) && !c.r.isCopy(elem) && c.rejectOptFieldValue(n, member.Base().ID(), member.Member().Text(), t) {
		return TyPoison
	}
	c.buildEdge(n, c.r.Types.Option(t))
	return c.r.Types.Option(t)
}
