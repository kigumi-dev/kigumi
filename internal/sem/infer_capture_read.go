package sem

import "kigumi/internal/syntax"

type heldUse struct {
	node syntax.NodeID
	res  reservation
}

// The lambda's reserved-place reads stay valid only while the local is never used beyond direct calls.
type heldLambda struct {
	pat  syntax.NodeID
	uses []heldUse
}

func (c *checker) readOnlyUse(top syntax.NodeID, above []syntax.NodeID) bool {
	if len(above) == 0 {
		return false
	}
	parent := above[len(above)-1]
	switch c.t.Kind(parent) {
	case syntax.BorrowExpr:
		return c.t.View(parent).AsBorrow().Flags()&syntax.FlagMut == 0
	case syntax.MemberExpr:
		if c.t.View(parent).AsMember().Base().ID() != top || len(above) < 2 {
			return false
		}
		call := above[len(above)-2]
		return c.t.Kind(call) == syntax.CallExpr && c.t.View(call).AsCall().Callee().ID() == parent && c.info.Calls[call].Recv == RecvSelf
	}
	return c.r.isCopy(c.vars.resolve(c.info.Types[top]))
}

func (c *checker) mutBorrowOperand(above []syntax.NodeID) bool {
	if len(above) == 0 {
		return false
	}
	parent := above[len(above)-1]
	return c.t.Kind(parent) == syntax.BorrowExpr && c.t.View(parent).AsBorrow().Flags()&syntax.FlagMut != 0
}

// A lambda that may outlive the reserved place keeps E2101; only a direct call or a call-only binding is safe.
func (c *checker) settleReadOnlyLambda(lambda syntax.NodeID, reads []heldUse) {
	if len(reads) == 0 {
		return
	}
	path := c.pathTo(c.t.Root, lambda)
	cur, i := lambda, len(path)-1
	for ; i >= 0 && c.t.Kind(path[i]) == syntax.Paren; i-- {
		cur = path[i]
	}
	if i >= 0 {
		switch par := path[i]; c.t.Kind(par) {
		case syntax.CallExpr:
			if c.t.View(par).AsCall().Callee().ID() == cur {
				return
			}
		case syntax.LetStmt:
			l := c.t.View(par).AsLet()
			if pat := l.Pattern().ID(); l.Init().ID() == cur && l.Else().ID() == 0 && c.t.Kind(pat) == syntax.PatBind {
				c.exitCheck.lambdas = append(c.exitCheck.lambdas, heldLambda{pat: pat, uses: reads})
				return
			}
		}
	}
	c.rejectHeldUses(reads)
}

func (c *checker) rejectHeldUses(uses []heldUse) {
	for _, u := range uses {
		c.reservedError(u.node, cCleanupReservedMove, u.res)
	}
}

func (c *checker) settleHeldLambdas() {
	for _, h := range c.exitCheck.lambdas {
		if local := c.info.Defs[h.pat]; local == 0 || c.usedBeyondCalls(c.t.Root, local, false) {
			c.rejectHeldUses(h.uses)
		}
	}
	c.exitCheck.lambdas = nil
}

func (c *checker) usedBeyondCalls(n syntax.NodeID, local EntityID, inLambda bool) bool {
	switch c.t.Kind(n) {
	case syntax.Ident:
		return c.info.Uses[n] == local
	case syntax.Lambda:
		inLambda = true
	case syntax.CallExpr:
		call := c.t.View(n).AsCall()
		if callee := c.t.Unparen(call.Callee().ID()); c.t.Kind(callee) == syntax.Ident && c.info.Uses[callee] == local {
			if inLambda {
				return true
			}
			for a := range call.Args().IDs() {
				if c.usedBeyondCalls(a, local, inLambda) {
					return true
				}
			}
			return false
		}
	}
	for ch := range c.t.View(n).Children() {
		if c.usedBeyondCalls(ch.ID(), local, inLambda) {
			return true
		}
	}
	return false
}
