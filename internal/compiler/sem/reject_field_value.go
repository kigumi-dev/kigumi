package sem

import "kigumi/internal/compiler/syntax"

// Only `move` takes a field out of an owned place; a call argument stays the one implicit move.
func (c *checker) rejectFieldValue(n syntax.NodeID) {
	for _, leaf := range c.tailLeaves(n, 0) {
		c.rejectFieldLeaf(c.t.Unparen(leaf))
	}
}

func (c *checker) rejectFieldLeaf(n syntax.NodeID) {
	if c.t.Kind(n) != syntax.MemberExpr || c.info.Uses[n] == 0 || c.r.Entities[c.info.Uses[n]].Kind != EntField {
		return
	}
	t := c.vars.resolve(c.info.Types[n])
	if t == 0 || t == TyPoison || c.r.isCopy(t) || c.r.Types.Kind(t) == KRef || c.indexRooted(n) || c.borrowedPlace(n) {
		return
	}
	root := c.fieldRoot(n)
	if root == 0 {
		return
	}
	if k := c.r.Entities[root].Kind; k != EntLocal && k != EntParam {
		return
	}
	name := c.placeName(n)
	c.errAtHelp(n, c.noCloneHelp(t, "write `move "+name+"` to take it out, or borrow it with `&"+name+"`"), cFieldValueMove, name, t, name, name)
}

// rejectBranchFieldArg: a plain `use(w.a)` stays an implicit move, but a field picked by an if / match / block tail is not tracked as one.
func (c *checker) rejectBranchFieldArg(a syntax.NodeID) {
	switch c.t.Kind(c.t.Unparen(a)) {
	case syntax.IfExpr, syntax.MatchExpr, syntax.Block:
		c.rejectFieldValue(a)
	}
}
