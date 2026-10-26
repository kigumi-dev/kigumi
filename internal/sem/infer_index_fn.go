package sem

import "kigumi/internal/syntax"

func (c *checker) explicitFnItem(n syntax.NodeID, fn EntityID, params []EntityID, items syntax.ListView) TypeID {
	var typeArgs []TypeID
	for i, a := range items.EnumerateIDs() {
		typeArgs = append(typeArgs, c.r.resolveExprGenericArg(c.f, c.scope, a, entAt(params, i)))
	}
	c.r.useEntity(c.f, n, fn)
	c.info.Uses[n] = fn
	sig, inst, _ := c.instantiateFn(n, fn, typeArgs, 0)
	c.fnItemHidden(n, fn, inst)
	c.errorAsCheck(n, fn, inst)
	c.deferFFIValueCheck(n, fn, inst, 0)
	c.touched = append(c.touched, n)
	if c.r.Fn(fn).Declared&EffAsync != 0 {
		sig = c.asyncValueFn(sig)
		c.noteAsyncValue(n, fn, sig)
	} else {
		c.valueEntry(n, fn, sig)
	}
	c.recordCtxValueRef(n, fn)
	return sig
}
