package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// While a loop's sites are rerooted, a holder rebound later in the loop reaches them unless it is declared inside the loop.
type loopReroot struct {
	loop, site syntax.NodeID
}

type loopSiteMark struct{ sites, calls int }

func (c *checker) markLoopSites() loopSiteMark {
	return loopSiteMark{len(c.exclusiveSites), len(c.outsideCalls)}
}

func (c *checker) rebindReaches(holder EntityID, rhs syntax.NodeID) bool {
	r := c.held.reroot
	if r.loop == 0 || c.t.Span(rhs).Start < c.t.Span(r.site).Start {
		return true
	}
	e := &c.r.Entities[holder]
	loop := c.t.Span(r.loop)
	return e.File != c.f || c.t.Span(e.Node).Start < loop.Start || c.t.Span(e.Node).Start >= loop.End
}

func (c *checker) rerootLoopSites(loop syntax.NodeID, m loopSiteMark) {
	saved := c.held.reroot
	defer func() { c.held.reroot = saved }()
	for i := min(m.sites, len(c.exclusiveSites)); i < len(c.exclusiveSites); i++ {
		s := &c.exclusiveSites[i]
		c.held.reroot = loopReroot{loop: loop, site: s.base}
		s.roots, s.lent = c.unionRoots(s.roots, s.lent, s.root, s.field)
	}
	for i := min(m.calls, len(c.outsideCalls)); i < len(c.outsideCalls); i++ {
		call := &c.outsideCalls[i]
		c.held.reroot = loopReroot{loop: loop, site: call.base}
		call.roots, call.lent = c.unionRoots(call.roots, call.lent, call.root, call.field)
	}
}

// A root both sides reach through different fields is taken whole.
func (c *checker) unionRoots(roots []EntityID, lent map[EntityID]EntityID, root, field EntityID) ([]EntityID, map[EntityID]EntityID) {
	more, moreLent := c.exclusiveRoots(root, field)
	out, outLent := slices.Clone(roots), map[EntityID]EntityID{}
	for r, f := range lent {
		outLent[r] = f
	}
	for _, r := range more {
		if !slices.Contains(roots, r) {
			out = append(out, r)
			if f := moreLent[r]; f != 0 {
				outLent[r] = f
			}
		} else if lent[r] != moreLent[r] {
			delete(outLent, r)
		}
	}
	return out, outLent
}
