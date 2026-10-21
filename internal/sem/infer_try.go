package sem

import "kigumi/internal/syntax"

// synthFail checks `fail e` against the enclosing function's error type;
// the value never returns.
func (c *checker) synthFail(n syntax.NodeID) TypeID {
	operand := c.t.View(n).AsFail().Value().ID()
	if c.cleanup > 0 {
		c.errAt(n, cCleanupControlFlow, "`fail`")
	}
	_, errType, ok := c.r.Types.IsResult(c.retType)
	if !ok {
		if c.lambdaRetUnknown() {
			c.errAt(n, cTryInLambdaUnknownRet)
		} else if c.retType != TyPoison {
			c.errAt(n, cFailOutsideResult, c.retType)
		}
		c.synth(operand)
		return TyNever
	}
	if operand == 0 {
		c.errAt(n, cReturnMissingValue, errType)
		return TyNever
	}
	c.check(operand, errType)
	if !c.fixedValue(operand, errType) {
		c.buildEdge(n, c.retType)
	}
	return TyNever
}

// synthTry checks postfix `?`.
func (c *checker) synthTry(n syntax.NodeID) TypeID {
	operand := c.t.View(n).AsTry().Operand().ID()
	tt := c.r.Types
	got := c.vars.resolve(c.synth(operand))
	if c.cleanup > 0 {
		c.errAt(n, cCleanupControlFlow, "`?`")
	}
	if c.leavesComptime(n, "`?`") {
		return TyPoison
	}
	if tt.Kind(got) == KUntyped {
		got = c.adopt(c.literalNode(operand), defaultOf(got))
	}
	if got == TyPoison {
		return TyPoison
	}
	borrowed := tt.Kind(got) == KRef
	if borrowed {
		got = tt.Node(got).Elem
	}
	if _, ok := tt.IsOption(got); ok {
		c.errAt(n, cTryOnOption)
		return TyPoison
	}
	val, e, ok := tt.IsResult(got)
	if !ok {
		c.errAt(n, cTryNotResult, got)
		return TyPoison
	}
	if borrowed && !c.r.isCopy(val) {
		c.errAt(n, cMoveOutOfBorrow, val)
	}
	if !borrowed {
		c.reservePayloadTake(operand, got, val)
	}
	return c.tryPropagate(n, got, val, e, borrowed || c.namedOperand(operand))
}

// tryPropagate checks that a `?` on a Result of val and error e can return its error from the enclosing function.
func (c *checker) tryPropagate(n syntax.NodeID, got, val, e TypeID, rebuilt bool) TypeID {
	tt := c.r.Types
	_, want, retOK := tt.IsResult(c.retType)
	if !retOK {
		if c.lambdaRetUnknown() {
			c.errAt(n, cTryInLambdaUnknownRet)
		} else if c.retType != TyPoison {
			c.errAt(n, cTryOutsideResult, c.retType)
		}
		return val
	}
	c.info.Calls[n] = CallInfo{Kind: CallTry}
	if rebuilt || got != c.vars.resolve(c.retType) {
		c.buildEdge(n, c.retType)
	}
	if e == want || e == TyPoison || want == TyPoison {
		return val
	}
	if tt.Kind(e) == KVar && c.vars.unify(e, want) {
		return val
	}
	if want == tt.errorType() && c.r.conforms(e, want, c.pkg).ok {
		c.info.Calls[n] = CallInfo{Kind: CallTry, Inst: []TypeID{e}}
		c.edge(EffectEdge{Kind: EdgeAlloc, Type: want, From: e, Node: n})
		return val
	}
	c.errAt(n, cTryErrorMismatch, e, want)
	return val
}

// The Result stays owned by the binding, so the Err is built anew rather than handed on.
func (c *checker) namedOperand(n syntax.NodeID) bool {
	_, ok := c.identPlaceEnt(n)
	return ok
}
