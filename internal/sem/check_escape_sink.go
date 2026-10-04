package sem

import "kigumi/internal/syntax"

// checkEscapeArg rejects passing a tainted callback as argument a unless the target is verified
// non-escaping. targetType backs the check when target has no entity, since only an explicit `escaping` there can mark it unsafe.
func (c *checker) checkEscapeArg(a syntax.NodeID, target EntityID, targetType TypeID, calleeName string) {
	rp := c.restrictedParam(a)
	if rp == 0 {
		return
	}
	if target != 0 {
		if c.r.nonEscapingParam(target) {
			return
		}
		if c.r.fnShapeOf(c.r.Entities[target].Type) == 0 {
			// Rejected, not unverified: a generic body is checked once for abstract T, with no per-call-site tracking.
			c.errAt(a, cEscapingForwardedOpaque, c.r.Entities[rp].Name, calleeName)
			return
		}
		c.errAt(a, cEscapingForwarded, c.r.Entities[rp].Name, calleeName)
		return
	}
	rt := c.vars.resolve(targetType)
	if c.r.Types.Kind(rt) == KFn && !c.r.Types.FnIsEscaping(rt) {
		return
	}
	c.errAt(a, cEscapingForwarded, c.r.Entities[rp].Name, calleeName)
}

func (c *checker) checkEscapeReturn(n syntax.NodeID) {
	if rp := c.restrictedParam(n); rp != 0 {
		c.errAt(n, cEscapingReturned, c.r.Entities[rp].Name)
	}
}

// checkEscapeTailReturn is checkEscapeReturn for the implicit tail value; a lambda body need not be a block.
// Runs regardless of the return type's shape since a tainted callback can nest inside a non-Fn-shaped aggregate.
func (c *checker) checkEscapeTailReturn(body syntax.NodeID) {
	if c.t.Kind(body) != syntax.Block {
		c.checkEscapeReturn(body)
		return
	}
	stmts := c.t.Children(body)
	if len(stmts) == 0 || c.t.Kind(stmts[len(stmts)-1]) != syntax.ExprStmt {
		return
	}
	c.checkEscapeReturn(syntax.NodeID(c.t.Nodes[stmts[len(stmts)-1]].Lhs))
}

// checkEscapeStore rejects storing a tainted callback into a field: an assignment, a record
// literal's field, or a variant's payload — all outlive the call.
func (c *checker) checkEscapeStore(n syntax.NodeID) {
	if rp := c.restrictedParam(n); rp != 0 {
		c.errAt(n, cEscapingStored, c.r.Entities[rp].Name)
	}
}
