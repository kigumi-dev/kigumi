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
	if len(info.Witnesses) > 0 {
		c.errAt(n, cWitnessFnValue, name)
	}
	if len(info.ConstParams) > 0 {
		c.errAt(n, cConstFnValue, name)
	}
}

// fnItemHidden binds the witnesses of a constrained generic into its function value; an async fn has no slot for them.
func (c *checker) fnItemHidden(n syntax.NodeID, fn EntityID, inst []TypeID) {
	info := c.r.Fn(fn)
	if len(info.ConstParams) > 0 {
		c.errAt(n, cConstFnValue, c.r.Entities[fn].Name)
	}
	switch {
	case len(info.Witnesses) == 0:
	case info.Declared&EffAsync != 0:
		c.errAt(n, cWitnessFnValue, c.r.Entities[fn].Name)
	default:
		c.info.Calls[n] = CallInfo{Kind: CallFnItem, Callee: fn, Inst: inst}
		c.deferWitnesses(n, fn, 0, inst)
		c.edge(EffectEdge{Kind: EdgeAlloc, Node: n, Target: fn, Why: whyBind})
	}
}

const whyBind = 13
