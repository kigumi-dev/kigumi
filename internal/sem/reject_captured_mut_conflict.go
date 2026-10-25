package sem

import (
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

// conservative by design, false positives accepted: a `mut self` call from outside
// the capturing closure is rejected; a call from inside it creates the cell, so it's allowed.
func (c *checker) checkCapturedMutConflict(base syntax.NodeID, root EntityID) {
	if c.capturedHere(root) {
		c.noteCaptureWrite(base, root)
		return
	}
	c.shareOutsideWrite(base, root)
	_, field := c.placeRoot(base)
	roots, lent := c.exclusiveRoots(root, field)
	c.outsideCalls = append(c.outsideCalls, outsideMutCall{base: base, root: root, field: field, roots: roots, lent: lent})
}

// shareOutsideWrite moves the closures that copied root onto its identity cell so a `&mut` or `mut self` write outside
// them reaches them; no closure writes here, so unlike an assignment it records no closure write site.
func (c *checker) shareOutsideWrite(n syntax.NodeID, root EntityID) {
	e := &c.r.Entities[root]
	c.markOpenWritten(root)
	if c.r.Types.Kind(e.Type) == KRef || !c.copyNow(e.Type) {
		return
	}
	c.noteLocalWrite(n, root)
	for _, site := range c.captureSites[root] {
		if site.ctx.list[site.idx].Mode == capCopy {
			site.ctx.list[site.idx].Mode = capCell
			e.Flags |= EfCapturedMut
		}
	}
}

func (c *checker) noteLocalWrite(n syntax.NodeID, ent EntityID) {
	if c.held.localWrites == nil {
		c.held.localWrites = map[EntityID][]syntax.NodeID{}
	}
	c.held.localWrites[ent] = append(c.held.localWrites[ent], n)
}

// writtenInLoopOf reports a write of ent before n in a loop that also holds n and is outside ent's declaration, so it
// runs again after the capture at n; a local declared in the loop body is a fresh variable each iteration.
func (c *checker) writtenInLoopOf(ent EntityID, n syntax.NodeID) bool {
	decl := c.r.Entities[ent].Node
	for _, w := range c.held.localWrites[ent] {
		pw, pn := c.t.Span(w).Start, c.t.Span(n).Start
		for _, l := range c.loopSpans {
			inside := func(p token.Pos) bool { return l.Start <= p && p < l.End }
			if inside(pw) && inside(pn) && (decl == 0 || c.r.Entities[ent].File != c.f || !inside(c.t.Span(decl).Start)) {
				return true
			}
		}
	}
	return false
}
