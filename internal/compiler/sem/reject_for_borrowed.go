package sem

import "kigumi/internal/compiler/syntax"

func (c *checker) rejectForOfBorrowed(head syntax.NodeID, iter, elem TypeID) {
	head = c.t.Unparen(head)
	if c.r.Types.Kind(iter) == KRef || c.r.isCopy(c.vars.resolve(elem)) || !c.pathMode(head).borrowed() {
		return
	}
	c.errAt(head, cForMovesBorrowed, c.takeName(head), c.vars.resolve(elem), c.takeName(head))
}
