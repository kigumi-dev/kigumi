package sem

import "kigumi/internal/syntax"

// adoptTails gives the untyped literals at the tails of an if / match / block target as well,
// so an untyped branch value agrees with the type the whole expression takes.
func (c *checker) adoptTails(n syntax.NodeID, target TypeID) {
	if n == 0 || c.r.Types.Kind(c.info.Types[n]) != KUntyped {
		return
	}
	var parts []syntax.NodeID
	switch c.t.Kind(n) {
	case syntax.Paren:
		parts = []syntax.NodeID{c.t.View(n).AsParen().Inner().ID()}
	case syntax.Block:
		if tail := c.t.View(n).AsBlock().Tail(); tail.Valid() {
			parts = []syntax.NodeID{tail.ID()}
		}
	case syntax.IfExpr:
		s := ifExpr(c.t, n)
		parts = []syntax.NodeID{s.Then, s.Else}
	case syntax.IfLetExpr:
		s := ifLet(c.t, n)
		parts = []syntax.NodeID{s.Then, s.Else}
	case syntax.ForExpr:
		parts = c.loopValues[n]
	case syntax.MatchExpr:
		for arm := range c.t.View(n).AsMatch().Arms().IDs() {
			parts = append(parts, matchArm(c.t, arm).Body)
		}
	}
	for _, p := range parts {
		if p != 0 && c.r.Types.Kind(c.info.Types[p]) == KUntyped {
			c.adopt(p, target)
		}
	}
}
