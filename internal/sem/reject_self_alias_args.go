package sem

import "kigumi/internal/syntax"

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
			a = syntax.NodeID(c.t.Nodes[a].Lhs)
		}
		if cand, ok := c.mutAliasArgOf(a); ok && cand.root == root && mutAliasStepsOverlap(steps, cand.steps) {
			c.errAt(cand.node, cMutAliasSelfArg)
		}
	}
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
