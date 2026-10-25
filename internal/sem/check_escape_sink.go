package sem

import "kigumi/internal/syntax"

// checkEscapeArg rejects passing a tainted callback as argument a unless the target is verified non-escaping.
// targetType backs the check when target has no entity; marked is the callee fn type's `escaping F` mark.
func (c *checker) checkEscapeArg(a syntax.NodeID, target EntityID, targetType TypeID, marked bool, calleeName string) {
	rp := c.restrictedParam(a)
	if rp == 0 || c.nonEscapingArg(target, targetType, marked) {
		return
	}
	if target != 0 && c.r.fnShapeOf(c.r.callbackType(&c.r.Entities[target])) == 0 {
		// Rejected, not unverified: a generic body is checked once for abstract T, with no per-call-site tracking.
		c.errAt(a, cEscapingForwardedOpaque, c.r.Entities[rp].Name, calleeName)
		return
	}
	c.errAt(a, cEscapingForwarded, c.r.Entities[rp].Name, calleeName, c.r.escapingForm(rp))
}

// nonEscapingArg: the parameter an argument binds to cannot keep it past the call.
func (c *checker) nonEscapingArg(target EntityID, targetType TypeID, marked bool) bool {
	if target != 0 {
		return c.r.nonEscapingParam(target)
	}
	rt := c.vars.resolve(targetType)
	return !marked && c.r.fnShapeOf(rt) != 0 && !c.r.Types.FnIsEscaping(rt)
}

func (c *checker) checkEscapeReturn(n syntax.NodeID) {
	if rp := c.restrictedParam(n); rp != 0 {
		c.errAt(n, cEscapingReturned, c.r.Entities[rp].Name, c.r.escapingForm(rp))
	}
}

// checkEscapeTailReturn is checkEscapeReturn for the implicit tail value; a lambda body need not be a block.
// Runs regardless of the return type's shape since a tainted callback can nest inside a non-Fn-shaped aggregate.
func (c *checker) checkEscapeTailReturn(body syntax.NodeID) {
	if c.t.Kind(body) != syntax.Block {
		c.checkEscapeReturn(body)
		return
	}
	tail := c.t.View(body).AsBlock().Tail()
	if !tail.Valid() {
		return
	}
	c.checkEscapeReturn(tail.ID())
}

// checkEscapeStore rejects storing a tainted callback into a field: an assignment, a record
// literal's field, or a variant's payload — all outlive the call.
func (c *checker) checkEscapeStore(n syntax.NodeID) {
	if rp := c.restrictedParam(n); rp != 0 {
		c.errAt(n, cEscapingStored, c.r.Entities[rp].Name, c.r.escapingForm(rp))
	}
}
