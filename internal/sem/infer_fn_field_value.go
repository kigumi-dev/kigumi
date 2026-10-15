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
		if c.r.fnLocalFieldRead == nil {
			c.r.fnLocalFieldRead = map[EntityID]syntax.NodeID{}
		}
		c.r.fnLocalFieldRead[local] = c.fnFieldReadNode(init)
	}
}

// fnFieldReadNode is where the field was read: a local holding it read it at its binding, however much later it is used.
func (c *checker) fnFieldReadNode(expr syntax.NodeID) syntax.NodeID {
	switch c.t.Kind(expr) {
	case syntax.Paren:
		return c.fnFieldReadNode(c.t.View(expr).AsParen().Inner().ID())
	case syntax.Ident:
		if at, ok := c.r.fnLocalFieldRead[c.info.Uses[expr]]; ok {
			return at
		}
	}
	return expr
}

func (c *checker) stageFnFieldHandoff(n, arg syntax.NodeID, param EntityID) {
	if param == 0 {
		return
	}
	if key, ok := c.fnFieldSrcKey(arg); ok {
		c.stageLateFieldEdge(EdgeFieldBind, key, param, n, c.fnFieldReadNode(arg))
		c.r.fieldPoisonForwards = append(c.r.fieldPoisonForwards, fieldPoisonForward{src: key, dst: param, file: c.f, node: n})
	}
}
