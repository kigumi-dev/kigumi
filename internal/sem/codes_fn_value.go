package sem

import "kigumi/internal/syntax"

var (
	cWitnessFnValue = defineCode("E1730", "witness-fn-value", "`{name}` has interface-bounded type parameters, which a function value cannot carry; call it directly or wrap the call in a closure")
	cConstFnValue   = defineCode("E1731", "const-fn-value", "`{name}` has const generic parameters, which a function value cannot carry; call it directly or wrap the call in a closure")
)

// rejectHiddenArgs: hidden trailing arguments have no slot in a function value.
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
