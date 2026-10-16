package sem

import (
	"slices"

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
		for _, site := range c.captureSites[root] {
			if site.ctx.list[site.idx].Mode != capCopy {
				return true
			}
		}
	}
	return false
}

// exclusiveRoots follows `let r = &mut place` and `let r = x` aliases; lent maps a root reached
// through a field place to that first field, the only part of it the access owns.
func (c *checker) exclusiveRoots(root, field EntityID) ([]EntityID, map[EntityID]EntityID) {
	roots := []EntityID{root}
	var lent map[EntityID]EntityID
	if field != 0 {
		lent = map[EntityID]EntityID{root: field}
	}
	for ent := root; len(roots) < 8; {
		next, nextField := c.lentFrom[ent], EntityID(0)
		if init, ok := c.letInit[ent]; ok && next == 0 {
			init = c.t.Unparen(init)
			switch c.t.Kind(init) {
			case syntax.BorrowExpr:
				next, nextField = c.placeRoot(c.t.View(init).AsBorrow().Operand().ID())
			case syntax.Ident:
				next = c.info.Uses[init]
			}
		}
		if next == 0 || slices.Contains(roots, next) || c.r.Entities[next].Kind != EntLocal && c.r.Entities[next].Kind != EntParam {
			break
		}
		if nextField != 0 {
			if lent == nil {
				lent = map[EntityID]EntityID{}
			}
			lent[next] = nextField
		}
		roots = append(roots, next)
		ent = next
	}
	return roots, lent
}

func (c *checker) placeRoot(n syntax.NodeID) (EntityID, EntityID) {
	p, ok := c.buildPlace(n)
	if !ok {
		return 0, 0
	}
	if len(p.Fields) == 0 {
		return p.Root, 0
	}
	return p.Root, p.Fields[0]
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
		c.settleOutsideCall(call)
	}
	for _, s := range c.exclusiveSites {
		c.settleExclusiveSite(s)
	}
	c.outsideCalls, c.exclusiveSites = nil, nil
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
		if c.lentFrom == nil {
			c.lentFrom = map[EntityID]EntityID{}
		}
		c.lentFrom[c.info.Defs[pat]] = root
	}
}
