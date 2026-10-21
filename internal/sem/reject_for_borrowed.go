package sem

import "kigumi/internal/syntax"

func (c *checker) rejectForOfBorrowed(head syntax.NodeID, iter, elem TypeID) {
	head = c.t.Unparen(head)
	if c.r.Types.Kind(iter) == KRef || c.r.isCopy(c.vars.resolve(elem)) || !c.borrowedPlace(head) {
		return
	}
	c.errAt(head, cForMovesBorrowed, c.placeName(head), c.vars.resolve(elem), c.placeName(head))
}
