package sem

import "kigumi/internal/syntax"

const maxLetLeafDepth = 8

// expandLetLeaves replaces a returned let-bound local that picks among values (`if`, `match`, a block) with those values.
func (c *checker) expandLetLeaves(leaves []syntax.NodeID, depth int) []syntax.NodeID {
	var out []syntax.NodeID
	for _, leaf := range leaves {
		init, ok := c.pickingInit(leaf)
		if !ok || depth >= maxLetLeafDepth {
			out = append(out, leaf)
			continue
		}
		out = append(out, c.expandLetLeaves(c.tailLeaves(init, 0), depth+1)...)
	}
	return out
}

func (c *checker) pickingInit(leaf syntax.NodeID) (syntax.NodeID, bool) {
	leaf = c.stripParen(leaf)
	if c.t.Kind(leaf) != syntax.Ident {
		return 0, false
	}
	ent := c.info.Uses[leaf]
	if ent == 0 || c.r.Entities[ent].Kind != EntLocal || c.r.Entities[ent].Flags&EfMut != 0 {
		return 0, false
	}
	init, ok := c.letInit[ent]
	if !ok {
		return 0, false
	}
	switch c.t.Kind(c.stripParen(init)) {
	case syntax.IfExpr, syntax.MatchExpr, syntax.Block:
		return init, true
	}
	return 0, false
}
