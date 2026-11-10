package sem

import "kigumi/internal/compiler/syntax"

// staticStoreFrees charges the allocator call a store through a `static mut`
// makes when it replaces a heap-owning value: the old buffer is released. The
// previous contents of a static are not knowable from one function — another
// handler or the main flow may have stored an owned buffer there — so the
// release must be assumed, and an export(interrupt) handler that performs it
// is rejected (E1080), as is one that calls a helper which does.
func (c *checker) staticStoreFrees(lhs syntax.NodeID, target TypeID) {
	if c.r.ownsHeap(c.vars.resolve(target)) {
		c.edge(EffectEdge{Kind: EdgeAlloc, Node: lhs})
	}
}
