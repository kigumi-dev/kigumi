package sem

import "kigumi/internal/syntax"

// checkReentrantCapture rejects aliasing a `mut self` call's exclusive receiver via a
// mutably-capturing closure, or a fn-shaped argument unless opaqueFnOrigin proves it capture-free.
func (c *checker) checkReentrantCapture(base syntax.NodeID, args []syntax.NodeID) {
	root := c.rootOf(base)
	if root == 0 {
		return
	}
	for _, a := range args {
		if c.t.Kind(a) == syntax.NamedArg {
			a = syntax.NodeID(c.t.Nodes[a].Lhs)
		}
		c.checkReentrantCaptureArg(a, root)
	}
}

func (c *checker) checkReentrantCaptureArg(a syntax.NodeID, root EntityID) {
	expr := a
	for c.t.Kind(expr) == syntax.Paren {
		expr = syntax.NodeID(c.t.Nodes[expr].Lhs)
	}
	t := c.vars.resolve(c.info.Types[expr])
	tt := c.r.Types
	if tt.Kind(t) == KClosure {
		if c.closureCapturesRoot(tt.Node(t).Ent, root, map[EntityID]bool{}) {
			c.errAt(a, cReentrantMutCapture, c.r.Entities[root].Name)
		}
		return
	}
	if !c.fnShaped(t) {
		return
	}
	if c.opaqueFnOrigin(expr, map[syntax.NodeID]bool{}) {
		c.errAt(a, cReentrantCaptureUnknown, c.r.Entities[root].Name)
	}
}

// opaqueFnOrigin traces expr through let-bound locals and aliases back to its source;
// a plain fn-typed expression is opaque except a non-closure Ident or capture-free lambda.
func (c *checker) opaqueFnOrigin(expr syntax.NodeID, seen map[syntax.NodeID]bool) bool {
	if seen[expr] {
		return false
	}
	seen[expr] = true
	switch c.t.Kind(expr) {
	case syntax.Paren:
		return c.opaqueFnOrigin(syntax.NodeID(c.t.Nodes[expr].Lhs), seen)
	case syntax.Ident:
		ent := c.info.Uses[expr]
		if init, ok := c.letInit[ent]; ok {
			return c.opaqueFnOrigin(init, seen)
		}
		return ent == 0 || c.r.Entities[ent].Kind != EntFn
	case syntax.Lambda:
		return len(c.r.closure(c.info.Defs[expr]).Captures) != 0
	}
	return true
}

func (c *checker) fnShaped(t TypeID) bool {
	tt := c.r.Types
	switch tt.Kind(t) {
	case KFn:
		return true
	case KParam:
		for _, con := range c.r.typeParam(tt.Node(t).Ent).Constraints {
			if con.Kind == CFn || con.Kind == CFnMut || con.Kind == CFnOnce {
				return true
			}
		}
	}
	return false
}

// closureCapturesRoot walks a closure's own captures and, for any captured local
// that is itself a closure, that closure's captures too.
func (c *checker) closureCapturesRoot(closure, root EntityID, seen map[EntityID]bool) bool {
	if seen[closure] {
		return false
	}
	seen[closure] = true
	for _, cap := range c.r.closure(closure).Captures {
		if cap.Local == root {
			if cap.Mode != capCopy {
				return true
			}
			continue
		}
		ct := c.r.Entities[cap.Local].Type
		if c.r.Types.Kind(ct) == KClosure && c.closureCapturesRoot(c.r.Types.Node(ct).Ent, root, seen) {
			return true
		}
	}
	return false
}
