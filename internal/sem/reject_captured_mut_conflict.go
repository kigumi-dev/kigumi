package sem

import "kigumi/internal/syntax"

// conservative by design, false positives accepted: a `mut self` call from outside
// the capturing closure is rejected; a call from inside it creates the cell, so it's allowed.
func (c *checker) checkCapturedMutConflict(base syntax.NodeID, root EntityID) {
	if c.capturedHere(root) {
		c.noteCaptureWrite(base, root)
		return
	}
	_, field := c.placeRoot(base)
	roots, lent := c.exclusiveRoots(root, field)
	c.outsideCalls = append(c.outsideCalls, outsideMutCall{base: base, root: root, roots: roots, lent: lent})
}
