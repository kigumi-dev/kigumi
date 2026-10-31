package sem

import "kigumi/internal/compiler/syntax"

// synth infers the type of an expression without an expected type; check
// pushes an expected type inward where the kind allows and coerces
// otherwise. Both memoize in Types.
func (c *checker) synth(n syntax.NodeID) TypeID {
	if n == 0 {
		return TyPoison
	}
	place, read := c.wantIndexPlace, c.held.wantRead
	c.wantIndexPlace, c.held.wantRead = false, false
	if t := c.info.Types[n]; t != 0 {
		return t
	}
	if root := c.optChainRoot(n); root != 0 {
		return c.setType(n, c.synthOptChain(n, root))
	}
	if (place || read) && c.t.Kind(n) == syntax.Paren {
		c.wantIndexPlace, c.held.wantRead = place, read
		return c.setType(n, c.synth(c.t.View(n).AsParen().Inner().ID()))
	}
	if place && c.t.Kind(n) == syntax.BracketExpr {
		if c.indexAsPlace == nil {
			c.indexAsPlace = map[syntax.NodeID]bool{}
		}
		c.indexAsPlace[n] = true
	}
	t := c.synthKind(n, 0)
	if !place && (c.t.Kind(n) == syntax.MemberExpr || c.t.Kind(n) == syntax.OptMemberExpr) {
		t = c.checkIndexFieldValue(n, t)
		if !read {
			c.noteMutFieldValue(n, t)
		}
	}
	return c.setType(n, t)
}

// synthPlace is synth for an expression read to extend a place rather than
// materialized as a value, so a non-Copy BracketExpr result skips indexType's Copy requirement.
func (c *checker) synthPlace(n syntax.NodeID) TypeID {
	c.wantIndexPlace = true
	return c.synth(n)
}

func (c *checker) check(n syntax.NodeID, want TypeID) TypeID {
	if n == 0 {
		return TyPoison
	}
	if want == 0 {
		return c.synth(n)
	}
	if t := c.info.Types[n]; t != 0 {
		return c.coerce(n, t, want)
	}
	switch c.t.Kind(n) {
	case syntax.CallExpr, syntax.WsCallExpr, syntax.RecordLit, syntax.TupleLit, syntax.Lambda, syntax.BorrowExpr:
		return c.coerce(n, c.setType(n, c.synthKind(n, want)), want)
	case syntax.IntLit, syntax.FloatLit, syntax.Paren, syntax.Block, syntax.IfExpr, syntax.IfLetExpr,
		syntax.MatchExpr, syntax.UnsafeExpr, syntax.AllocatorExpr, syntax.ContextExpr, syntax.ContractExpr, syntax.AsmExpr:
		return c.setType(n, c.synthKind(n, want))
	case syntax.Ident, syntax.MemberExpr:
		if c.r.Types.Kind(c.vars.resolve(want)) == KFn {
			if t, ok := c.overloadValue(n, want); ok {
				return c.coerce(n, c.setType(n, t), want)
			}
		}
	case syntax.MoveExpr:
		if c.t.Kind(c.t.Unparen(c.t.View(n).AsMove().Operand().ID())) == syntax.Lambda {
			return c.coerce(n, c.setType(n, c.synthKind(n, want)), want)
		}
	case syntax.Unary, syntax.Binary:
		if t, ok := c.checkOperator(n, want); ok {
			return t
		}
	}
	got := c.synth(n)
	c.hintMethodValue(got, want)
	return c.coerce(n, got, want)
}

// synthKind dispatches on node kind; want is 0 in synth mode.
func (c *checker) synthKind(n syntax.NodeID, want TypeID) TypeID {
	if root := c.optChainRoot(n); root != 0 {
		return c.synthOptChain(n, root)
	}
	node := c.t.View(n)
	switch node.Kind() {
	case syntax.Ident:
		return c.synthIdent(n)
	case syntax.IntLit, syntax.FloatLit:
		return c.synthLiteral(n, want)
	case syntax.CharLit:
		if _, ok := syntax.DecodeChar(node.Tok().Text()); !ok {
			c.errAt(n, cCharInvalid)
		}
		return TyChar
	case syntax.BoolLit:
		return TyBool
	case syntax.StringLit:
		return c.synthString(n)
	case syntax.ByteStringLit:
		return TyBytes
	case syntax.ShellLit:
		return c.synthShell(n)
	case syntax.Paren:
		return c.check(node.AsParen().Inner().ID(), want)
	case syntax.Unary:
		return c.synthUnary(n, want)
	case syntax.Binary:
		return c.synthBinary(n, want)
	case syntax.IsExpr:
		return c.synthIs(n)
	case syntax.CallExpr, syntax.WsCallExpr:
		return c.synthCall(n, want)
	case syntax.BracketExpr:
		return c.synthBracket(n)
	case syntax.MemberExpr:
		return c.synthMember(n)
	case syntax.OptMemberExpr:
		return c.synthOptMember(n)
	case syntax.TryExpr:
		return c.synthTry(n)
	case syntax.RecordLit:
		return c.synthRecord(n, want)
	case syntax.TupleLit:
		return c.synthTuple(n, want)
	case syntax.UnitLit:
		return TyUnit
	case syntax.DefaultArg:
		return c.synthDefaultArg(n)
	case syntax.Lambda:
		return c.synthLambda(n, want)
	case syntax.Block:
		return c.synthBlock(n, want)
	case syntax.IfExpr:
		return c.synthIf(n, want)
	case syntax.IfLetExpr:
		return c.synthIfLet(n, want)
	case syntax.MatchExpr:
		return c.synthMatch(n, want)
	case syntax.ForExpr:
		return c.synthFor(n)
	case syntax.ReturnExpr:
		return c.synthReturn(n)
	case syntax.FailExpr:
		return c.synthFail(n)
	case syntax.BreakExpr, syntax.ContinueExpr:
		return c.synthBreak(n)
	case syntax.AwaitExpr:
		return c.synthAwait(n)
	case syntax.ComptimeExpr:
		return c.synthComptime(n)
	case syntax.AsmExpr:
		return c.synthAsm(n, want)
	case syntax.UnsafeExpr:
		c.edge(EffectEdge{Kind: EdgeUnsafe, Node: n})
		c.dropUnstableFacts()
		c.unsafe++
		t := c.check(node.AsUnsafe().Body().ID(), want)
		c.unsafe--
		return t
	case syntax.AllocatorExpr:
		return c.synthAllocator(n, want)
	case syntax.ContextExpr:
		return c.synthContext(n, want)
	case syntax.ContractExpr:
		return c.synthContract(n, want)
	case syntax.BorrowExpr:
		return c.synthBorrow(n, want)
	case syntax.MoveExpr:
		return c.synthMove(n, want)
	case syntax.Spread:
		c.errAt(n, cSpreadNonVariadic, "this call")
		c.synth(node.AsSpread().Value().ID())
		return TyPoison
	case syntax.TypeFn, syntax.TypePtr:
		c.errAt(n, cTypeExprAsValue)
		return TyPoison
	}
	c.errAt(n, cNotSupportedYet, node.Kind().String())
	return TyPoison
}

func (c *checker) notSupported(n syntax.NodeID, what string) TypeID {
	c.errAt(n, cNotSupportedYet, what)
	return TyPoison
}

// edge records an effect fact for the effects pass.
func (c *checker) edge(e EffectEdge) {
	e.File = c.f
	e.Scoped = c.allocDepth > 0
	*c.edges = append(*c.edges, e)
	if e.Kind == EdgeCall {
		c.recordCtxCallEdge(e.Target, e.Node)
	}
}
