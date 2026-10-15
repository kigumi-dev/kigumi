package sem

import "kigumi/internal/syntax"

func (c *checker) fnFieldSrcKey(expr syntax.NodeID) (fieldFnKey, bool) {
	switch c.t.Kind(expr) {
	case syntax.Paren:
		return c.fnFieldSrcKey(c.t.View(expr).AsParen().Inner().ID())
	case syntax.Ident:
		key, ok := c.r.fnLocalField[c.info.Uses[expr]]
		return key, ok
	case syntax.MemberExpr:
		if key, ok := c.knownFieldCallKey(expr); ok && c.r.fnTypedFieldKey(key) {
			return key, true
		}
	}
	return fieldFnKey{}, false
}

// The field may only be poisoned once every body is checked, so the local carries its key.
func (c *checker) trackFnFieldLocal(init syntax.NodeID, local EntityID) {
	if key, ok := c.fnFieldSrcKey(init); ok {
		if c.r.fnLocalField == nil {
			c.r.fnLocalField = map[EntityID]fieldFnKey{}
		}
		c.r.fnLocalField[local] = key
	}
}

func (c *checker) stageFnFieldHandoff(n, arg syntax.NodeID, param EntityID) {
	if param == 0 {
		return
	}
	if key, ok := c.fnFieldSrcKey(arg); ok {
		c.stageLateFieldEdge(EdgeFieldBind, key, param, n)
		c.r.fieldPoisonForwards = append(c.r.fieldPoisonForwards, fieldPoisonForward{src: key, dst: param, file: c.f, node: n})
	}
}
