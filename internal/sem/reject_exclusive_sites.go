package sem

import (
	"kigumi/internal/syntax"
)

type exclusiveSite struct {
	base  syntax.NodeID
	args  []syntax.NodeID
	root  EntityID
	roots []EntityID
	lent  map[EntityID]EntityID
	// fnArgs: the strict fn-shaped argument rule applies to `mut self` calls only.
	fnArgs bool
}

type outsideMutCall struct {
	base  syntax.NodeID
	root  EntityID
	roots []EntityID
	lent  map[EntityID]EntityID
	// byRef: the access is a `&mut` argument, not a `mut self` receiver.
	byRef bool
}

func (c *checker) noteMutCapture(n syntax.NodeID, root EntityID) {
	if c.mutCaptures == nil {
		c.mutCaptures = map[EntityID][]syntax.NodeID{}
	}
	c.mutCaptures[root] = append(c.mutCaptures[root], n)
}

func (c *checker) rootsHeldByClosure(roots []EntityID) bool {
	for _, root := range roots {
		for _, h := range c.holders(root) {
			if h.mode != capCopy {
				return true
			}
		}
	}
	for local := range c.captureSites {
		if c.borrowsRoot(local, &heldResult{roots: roots}) {
			return true
		}
	}
	return false
}

func (c *checker) sharesLoop(a, b syntax.NodeID) bool {
	pa, pb := c.t.Span(a).Start, c.t.Span(b).Start
	for _, l := range c.loopSpans {
		if l.Start <= pa && pa < l.End && l.Start <= pb && pb < l.End {
			return true
		}
	}
	return false
}

func (c *checker) settleExclusive() {
	for _, call := range c.outsideCalls {
		c.settleOrDefer(call.roots, func() { c.settleOutsideCall(call) })
	}
	for _, s := range c.exclusiveSites {
		c.settleOrDefer(s.roots, func() { c.settleExclusiveSite(s) })
	}
	c.outsideCalls, c.exclusiveSites = nil, nil
}

func (c *checker) settleOrDefer(roots []EntityID, settle func()) {
	if c.deferredToContext(roots) {
		c.r.ctxDeferred = append(c.r.ctxDeferred, settle)
		return
	}
	settle()
}

func (c *checker) noteMutBorrowSites(args []syntax.NodeID) {
	for i, a := range args {
		root, field := c.mutBorrowRoot(a)
		if root == 0 {
			continue
		}
		var others []syntax.NodeID
		for j, o := range args {
			if j != i {
				others = append(others, o)
			}
		}
		roots, lent := c.exclusiveRoots(root, field)
		c.exclusiveSites = append(c.exclusiveSites, exclusiveSite{base: a, args: others, root: root, roots: roots, lent: lent})
		if !c.capturedHere(root) {
			c.outsideCalls = append(c.outsideCalls, outsideMutCall{base: a, root: root, roots: roots, lent: lent, byRef: true})
		}
	}
}

func (c *checker) mutBorrowRoot(a syntax.NodeID) (EntityID, EntityID) {
	a = c.t.Unparen(a)
	switch c.t.Kind(a) {
	case syntax.BorrowExpr:
		if c.t.View(a).AsBorrow().Flags()&syntax.FlagMut != 0 {
			return c.placeRoot(c.t.View(a).AsBorrow().Operand().ID())
		}
	case syntax.Ident:
		tt := c.r.Types
		if t := c.vars.resolve(c.info.Types[a]); tt.Kind(t) == KRef && tt.Node(t).Flags&flagMut != 0 {
			return c.info.Uses[a], 0
		}
	}
	return 0, 0
}

func (c *checker) noteLoopAlias(pat, head syntax.NodeID) {
	head = c.t.Unparen(head)
	if c.t.Kind(pat) != syntax.PatBind || c.t.Kind(head) != syntax.BorrowExpr || c.t.View(head).AsBorrow().Flags()&syntax.FlagMut == 0 {
		return
	}
	if root := c.rootOf(c.t.View(head).AsBorrow().Operand().ID()); root != 0 {
		c.setLentFrom(c.info.Defs[pat], lentRef{root: root})
	}
}

func (c *checker) setLentFrom(ent EntityID, from lentRef) {
	if c.lentFrom == nil {
		c.lentFrom = map[EntityID]lentRef{}
	}
	c.lentFrom[ent] = from
}
