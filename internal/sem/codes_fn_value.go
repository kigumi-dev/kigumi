package sem

import "kigumi/internal/syntax"

var (
	cWitnessFnValue = defineCode("E1730", "witness-fn-value", "`{name}` has interface-bounded type parameters, which a function value cannot carry; call it directly or wrap the call in a closure")
	cConstFnValue   = defineCode("E1731", "const-fn-value", "`{name}` has const generic parameters, which a function value cannot carry; call it directly or wrap the call in a closure")
	cAllocBind      = defineCode("E1820", "alloc-bind-fn-value", "binding the witnesses of `{name}` into a function value allocates; call it directly or bind it outside the `noalloc` body")
)

// rejectHiddenArgs: hidden trailing arguments have no slot in a method value.
func (c *checker) rejectHiddenArgs(n syntax.NodeID, fn EntityID) {
	info := c.r.Fn(fn)
	name := c.r.Entities[fn].Name
	if len(info.Witnesses) > 0 && !c.bindableMethodValue(fn) {
		c.errAt(n, cWitnessFnValue, name)
	}
	if len(info.ConstParams) > 0 {
		c.errAt(n, cConstFnValue, name)
	}
}

// bindableMethodValue: a method whose type parameters carry the bounds can bind their witnesses; a requirement has no slot for them.
func (c *checker) bindableMethodValue(fn EntityID) bool {
	return c.r.Entities[c.r.Entities[fn].Parent].Kind != EntInterface
}

// copyableBoundValue rejects a witness-bound method value on a move-only receiver before instantiating, which would only add an inference error.
func (c *checker) copyableBoundValue(n syntax.NodeID, fn EntityID, recv TypeID) bool {
	if len(c.r.Fn(fn).Witnesses) == 0 || !c.bindableMethodValue(fn) || c.r.isCopy(recv) {
		return true
	}
	c.errAt(n, cMethodValueMoveOnly, recv)
	return false
}

// bindOwnerBoundValue binds the witnesses a method takes from its owner type's bounds, which the receiver type already fixes.
func (c *checker) bindOwnerBoundValue(n syntax.NodeID, fn EntityID, recv TypeID) bool {
	if !c.copyableBoundValue(n, fn, recv) {
		return false
	}
	c.bindMethodWitnesses(n, fn, recv, nil)
	return true
}

func (c *checker) bindMethodWitnesses(n syntax.NodeID, fn EntityID, recv TypeID, inst []TypeID) {
	if len(c.r.Fn(fn).Witnesses) == 0 || !c.bindableMethodValue(fn) {
		return
	}
	c.deferWitnesses(n, fn, recv, inst)
	c.edge(EffectEdge{Kind: EdgeAlloc, Node: n, Target: fn, Why: whyBind})
}

// fnItemHidden binds the witnesses of a constrained generic into its function value.
func (c *checker) fnItemHidden(n syntax.NodeID, fn EntityID, inst []TypeID) {
	info := c.r.Fn(fn)
	if len(info.ConstParams) > 0 {
		c.errAt(n, cConstFnValue, c.r.Entities[fn].Name)
	}
	if len(info.Witnesses) > 0 {
		c.info.Calls[n] = CallInfo{Kind: CallFnItem, Callee: fn, Inst: inst}
		c.deferWitnesses(n, fn, 0, inst)
		c.edge(EffectEdge{Kind: EdgeAlloc, Node: n, Target: fn, Why: whyBind})
	}
}

const whyBind = 13
