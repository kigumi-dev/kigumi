package sem

import "kigumi/internal/compiler/syntax"

// Merged once every body is checked, so a later write to from still reaches to.
type fnFieldUnion struct {
	from, to fieldFnKey
	file     FileID
	node     syntax.NodeID
}

// A param-rooted source is left to poison: its fns come from callers, which a merge here never sees.
func (c *checker) stageFnFieldCopy(to fieldFnKey, init syntax.NodeID) bool {
	from, ok := c.fnFieldSrcKey(init)
	if !ok || c.r.Entities[c.r.rootOf(from.Ent)].Kind == EntParam {
		return false
	}
	if from != to {
		c.r.fnFieldUnions = append(c.r.fnFieldUnions, fnFieldUnion{from: from, to: to, file: c.f, node: init})
	}
	return true
}
