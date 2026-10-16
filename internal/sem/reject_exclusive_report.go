package sem

import (
	"kigumi/internal/diag"
	"kigumi/internal/syntax"
)

func (c *checker) accessNote(d diag.Diagnostic, s exclusiveSite, name string) diag.Diagnostic {
	return d.WithNote(c.loc(s.base), "`"+name+"` is exclusively borrowed by this call")
}

func (c *checker) captureNote(d diag.Diagnostic, closure EntityID, name string) diag.Diagnostic {
	if cl := &c.r.Entities[closure]; cl.File == c.f && cl.Node != 0 {
		d = d.WithNote(c.loc(cl.Node), "this closure captures `"+name+"`")
	}
	return d
}

func (c *checker) errConflictCapture(arg syntax.NodeID, s exclusiveSite, h *heldResult, direct bool) {
	closure := h.conflict
	name := c.r.Entities[h.hit].Name
	if direct {
		d, ok := c.r.diagAt(c.f, arg, cReentrantMutCapture, name)
		if ok {
			c.r.emitDiag(c.f, c.captureNote(c.accessNote(d, s, name), closure, name))
		}
		return
	}
	d, ok := c.r.diagAt(c.f, s.base, cReentrantHeldCapture, name)
	if ok {
		d = d.WithNote(c.loc(arg), "this argument holds a closure")
		c.r.emitDiag(c.f, c.captureNote(d, closure, name))
	}
}

func (c *checker) errUnknownCapture(arg syntax.NodeID, s exclusiveSite) {
	name := c.r.Entities[s.root].Name
	if d, ok := c.r.diagAt(c.f, arg, cReentrantCaptureUnknown, name); ok {
		c.r.emitDiag(c.f, c.accessNote(d, s, name))
	}
}

func (c *checker) errHiddenCapture(arg syntax.NodeID, s exclusiveSite, h *heldResult) {
	name := c.r.Entities[s.root].Name
	d, ok := c.r.diagAt(c.f, arg, cReentrantHiddenCapture, h.lostWhat, name)
	if !ok {
		return
	}
	d = c.accessNote(d, s, name)
	if h.lostNode != 0 && h.lostNode != c.t.Unparen(arg) {
		d = d.WithNote(c.loc(h.lostNode), "its closures are not visible from here")
	}
	c.r.emitDiag(c.f, d)
}

func (c *checker) errCapturedMutConflict(call outsideMutCall, root EntityID, w syntax.NodeID) {
	name := c.r.Entities[root].Name
	code := cCapturedMutSelfConflict
	if call.byRef {
		code = cCapturedMutBorrowConflict
	}
	if d, ok := c.r.diagAt(c.f, call.base, code, name); ok {
		c.r.emitDiag(c.f, d.WithNote(c.loc(w), "`"+name+"` is shared with a closure that writes it here"))
	}
}

func (c *checker) errStoredCapture(call outsideMutCall, root, closure EntityID) {
	name := c.r.Entities[root].Name
	if d, ok := c.r.diagAt(c.f, call.base, cReentrantStoredCapture, name); ok {
		c.r.emitDiag(c.f, c.captureNote(d, closure, name))
	}
}
