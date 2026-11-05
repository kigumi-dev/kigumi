package sem

import "kigumi/internal/compiler/syntax"

// checkSelfAliasArgs rejects a scalar `&mut` argument rooted in this call's
// `self`/`mut self` receiver; async calls are skipped since cMutPlaceAsyncArg already rejects every scalar place argument there.
func (c *checker) checkSelfAliasArgs(base syntax.NodeID, recv RecvKind, async bool, args []syntax.NodeID) {
	if async || recv != RecvMut && recv != RecvSelf {
		return
	}
	root, steps, ok := c.mutAliasChain(base)
	if !ok {
		return
	}
	for _, a := range args {
		if c.t.Kind(a) == syntax.NamedArg {
			a = c.t.View(a).AsNamedArg().Value().ID()
		}
		for _, cand := range c.argBorrows(a) {
			// A shared receiver only conflicts with a mutable borrow of its
			// storage; a shared borrow reads the same value harmlessly.
			if recv == RecvSelf && !cand.mut {
				continue
			}
			if cand.root == root && mutAliasStepsOverlap(steps, cand.steps) {
				c.errAt(a, cMutAliasSelfArg)
				break
			}
		}
	}
}

// argBorrows lists the borrows an argument may yield, including the branches of
// an if/match/block; a branch the checker cannot resolve still aliases the receiver.
func (c *checker) argBorrows(a syntax.NodeID) []mutAliasArg {
	leaves := []syntax.NodeID{a}
	switch c.t.Kind(c.t.Unparen(a)) {
	case syntax.IfExpr, syntax.IfLetExpr, syntax.MatchExpr, syntax.Block:
		leaves = c.valueLeaves(a)
	}
	var out []mutAliasArg
	seen := map[syntax.NodeID]bool{}
	for _, leaf := range leaves {
		if cand, ok := c.borrowAliasArgOf(c.t.Unparen(leaf), false); ok && !seen[cand.node] {
			seen[cand.node] = true
			out = append(out, cand)
		}
	}
	return out
}

// mutAliasStepsOverlap reports whether a and b, chains from the same root,
// overlap: one is a prefix of the other, and an unresolved index can't be proven apart.
func mutAliasStepsOverlap(a, b []mutAliasStep) bool {
	n := min(len(a), len(b))
	for i := range n {
		sa, sb := a[i], b[i]
		if sa.index != sb.index {
			return false
		}
		if !sa.index {
			if sa.field != sb.field {
				return false
			}
			continue
		}
		if sa.lit && sb.lit && sa.val != sb.val {
			return false
		}
	}
	return true
}
