package sem

import "kigumi/internal/syntax"

// Judged in settleExclusive, once every closure's capture mode is final.
func (c *checker) checkReentrantCapture(base syntax.NodeID, args []syntax.NodeID) {
	root, field := c.placeRoot(base)
	if root == 0 {
		return
	}
	roots, lent := c.exclusiveRoots(root, field)
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
		if hidden := c.opaqueFnOrigin(expr, s, map[syntax.NodeID]bool{}); hidden != 0 {
			c.errUnknownCapture(a, s, hidden)
		}
		return
	}
	if !c.rootsHeldByClosure(s.roots) {
		return
	}
	h := c.newHeldResult(s.roots, s.lent)
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

// opaqueFnOrigin returns the node whose captures stay hidden after tracing expr through lets, or 0 when they are visible.
func (c *checker) opaqueFnOrigin(expr syntax.NodeID, s exclusiveSite, seen map[syntax.NodeID]bool) syntax.NodeID {
	if seen[expr] {
		return 0
	}
	seen[expr] = true
	hidden := func(opaque bool) syntax.NodeID {
		if opaque {
			return expr
		}
		return 0
	}
	switch c.t.Kind(expr) {
	case syntax.Paren:
		return c.opaqueFnOrigin(c.t.View(expr).AsParen().Inner().ID(), s, seen)
	case syntax.Ident:
		ent := c.info.Uses[expr]
		if init, ok := c.letInit[ent]; ok {
			return c.opaqueFnOrigin(init, s, seen)
		}
		return hidden(ent == 0 || c.r.Entities[ent].Kind != EntFn)
	case syntax.Lambda:
		return hidden(len(c.r.closure(c.info.Defs[expr]).Captures) != 0)
	case syntax.MemberExpr:
		if c.isMethodValue(expr) {
			return hidden(!c.methodValueApart(expr, s))
		}
	}
	return expr
}

// A method value on a plain local copies its receiver; on any other place it keeps pointing at that place.
func (c *checker) methodValueApart(expr syntax.NodeID, s exclusiveSite) bool {
	base := c.t.Unparen(c.t.View(expr).AsMember().Base().ID())
	recv := c.vars.resolve(c.info.Types[base])
	if ent, ok := c.identPlaceEnt(base); ok && c.t.Kind(base) == syntax.Ident && c.r.Entities[ent].Kind == EntLocal && c.r.isCopy(recv) {
		return true
	}
	root, field := c.placeRoot(base)
	if root == 0 {
		return false
	}
	roots, lent := c.exclusiveRoots(root, field)
	return !placesOverlap(roots, lent, s.roots, s.lent)
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
