package sem

import "kigumi/internal/syntax"

// resultPathOf returns the fields from this fn's result down to key, when key is a field of the result or of a record inside it.
func (c *checker) resultPathOf(key fieldFnKey) ([]EntityID, bool) {
	res, ok := c.r.fnResultEnts[c.fn]
	if !ok {
		return nil, false
	}
	root, path := c.r.nestedPath(key.Ent)
	if root != res {
		return nil, false
	}
	return append(path, key.Field), true
}

// noteResultFieldParam records a result field initialized from a param place, so each caller's argument flows into that field.
func (c *checker) noteResultFieldParam(key fieldFnKey, src EntityID, init syntax.NodeID) bool {
	into, ok := c.resultPathOf(key)
	return ok && c.noteResultParamAt(into, src, init)
}

// noteLocalResultParams does the same for a returned local whose record fields were initialized from param places.
func (c *checker) noteLocalResultParams(local EntityID, t TypeID, into []EntityID, leaf syntax.NodeID, depth int) {
	if depth >= maxNestedFnDepth {
		return
	}
	for _, f := range c.r.recordFieldsOf(t) {
		ft := c.r.Entities[f].Type
		if c.r.fnTypedType(ft) || !c.r.holdsFn(ft, 0) {
			continue
		}
		key := fieldFnKey{Ent: local, Field: f}
		at := append(append([]EntityID(nil), into...), f)
		if src := c.r.fieldWitnessOf[key]; src != 0 && c.noteResultParamAt(at, src, leaf) {
			continue
		}
		c.noteLocalResultParams(c.r.nestedFnOwner(key), ft, at, leaf, depth+1)
	}
}

func (c *checker) noteParamReassign(root EntityID, rhs syntax.NodeID) {
	if c.r.Entities[root].Kind != EntParam || len(c.lambdas) > 0 {
		return
	}
	if c.paramReassign == nil {
		c.paramReassign = map[EntityID][]syntax.NodeID{}
	}
	c.paramReassign[root] = append(c.paramReassign[root], rhs)
}

// recordParamReassigns traces a returned param through the values it was wholly reassigned to.
func (c *checker) recordParamReassigns(ret, src EntityID, depth int) {
	root, path := c.r.nestedPath(src)
	if len(path) != 0 || depth >= maxLetLeafDepth {
		return
	}
	for _, rhs := range c.paramReassign[root] {
		for _, leaf := range c.expandLetLeaves(c.tailLeaves(rhs, 0), 0) {
			c.recordResultLeaf(ret, leaf, depth+1)
		}
	}
}
