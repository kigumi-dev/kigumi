package sem

import "kigumi/internal/syntax"

// Judged in settleExclusive, once every closure's capture mode is final.
func (c *checker) checkReentrantCapture(base syntax.NodeID, args []syntax.NodeID) {
	root := c.rootOf(base)
	if root == 0 {
		return
	}
	roots, lent := c.exclusiveRoots(root, 0)
	c.exclusiveSites = append(c.exclusiveSites, exclusiveSite{base: base, args: args, root: root, roots: roots, lent: lent, fnArgs: true})
}

func (c *checker) settleExclusiveSite(s exclusiveSite) {
	for _, a := range s.args {
		if c.t.Kind(a) == syntax.NamedArg {
			a = c.t.View(a).AsNamedArg().Value().ID()
		}
		c.checkReentrantCaptureArg(a, s)
	}
}

func (c *checker) checkReentrantCaptureArg(a syntax.NodeID, s exclusiveSite) {
	expr := c.t.Unparen(a)
	t := c.vars.resolve(c.info.Types[expr])
	if s.fnArgs && c.fnShaped(t) {
		if c.opaqueFnOrigin(expr, map[syntax.NodeID]bool{}) {
			c.errUnknownCapture(a, s)
		}
		return
	}
	if !c.rootsHeldByClosure(s.roots) {
		return
	}
	h := newHeldResult(s.roots, s.lent)
	c.heldOf(expr, h, map[syntax.NodeID]bool{})
	switch {
	case h.conflict != 0 && c.r.Types.Kind(t) == KClosure:
		c.errConflictCapture(a, s, h, true)
	case h.conflict != 0:
		c.errConflictCapture(a, s, h, false)
	case h.lost:
		c.errHiddenCapture(a, s, h)
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
		return c.opaqueFnOrigin(c.t.View(expr).AsParen().Inner().ID(), seen)
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
