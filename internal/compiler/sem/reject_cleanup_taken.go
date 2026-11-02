package sem

import "kigumi/internal/compiler/syntax"

// rejectCleanupReadsTaken: a field the result (or a `fail` value, which also runs `defer fail`) takes
// out of an owned binding is moved by the time a cleanup reading it runs, as with an explicit `move`.
func (c *checker) rejectCleanupReadsTaken(body syntax.NodeID) {
	for _, leaf := range c.resultLeaves(body) {
		c.rejectCleanupReadsLeaf(body, leaf, false)
	}
	for _, leaf := range c.exits.fails {
		c.rejectCleanupReadsLeaf(body, leaf, true)
	}
}

func (c *checker) rejectCleanupReadsLeaf(body, leaf syntax.NodeID, fail bool) {
	for _, n := range c.valueLeaves(leaf) {
		if !c.fieldMember(n) || c.pathMode(n) != pathOwned || !c.ownsType(c.vars.resolve(c.info.Types[n])) {
			continue
		}
		if p, ok := c.placeUnder(n); ok {
			c.rejectDeferReads(body, n, p, fail)
		}
	}
}

// rejectDeferReads looks at the cleanups registered before at in a block that encloses it: only
// those run after at hands its value on.
func (c *checker) rejectDeferReads(n, at syntax.NodeID, taken reservedPlace, fail bool) bool {
	switch c.t.Kind(n) {
	case syntax.NoneKind, syntax.Lambda:
		return false
	case syntax.Binary:
		if hit, arm, ok := c.orHandler(n); ok {
			return c.rejectDeferReads(hit, at, taken, fail) || c.rejectDeferReads(arm, at, taken, fail)
		}
	case syntax.Block:
		if !c.within(at, n) {
			return false
		}
		for st := range c.t.View(n).Children() {
			k := st.Kind()
			if (k == syntax.DeferStmt || fail && k == syntax.DeferFailStmt) && c.t.Span(st.ID()).End <= c.t.Span(at).Start {
				if read := c.readOf(st.ID(), taken); read != 0 {
					c.errAt(read, cCleanupReadsMoved, c.placeLabel(taken))
					return true
				}
			}
		}
	}
	for ch := range c.t.View(n).Children() {
		if c.rejectDeferReads(ch.ID(), at, taken, fail) {
			return true
		}
	}
	return false
}

// readOf is the first place under n that overlaps taken, or 0.
func (c *checker) readOf(n syntax.NodeID, taken reservedPlace) syntax.NodeID {
	if k := c.t.Kind(n); k == syntax.MemberExpr || k == syntax.Ident {
		if p, ok := c.placeUnder(n); ok {
			if p.ent == taken.ent && fieldPathsOverlap(p.fields, taken.fields) {
				return n
			}
			return 0
		}
	}
	for ch := range c.t.View(n).Children() {
		if r := c.readOf(ch.ID(), taken); r != 0 {
			return r
		}
	}
	return 0
}
