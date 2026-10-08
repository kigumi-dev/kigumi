package sem

func (c *checker) fixedResultOpen(p pendingInline, op string) bool {
	return (op == "at" || op == "withMut") && c.r.Types.ContainsVar(c.vars.resolve(c.info.Types[p.call]))
}

func (c *checker) chargeFixedResidue(p pendingInline, op string, recv TypeID) {
	if op != "at" && op != "withMut" {
		return
	}
	scoped := false
	for _, e := range (*p.edges)[p.from:p.to] {
		if e.Kind == EdgeCall && e.Target == p.fn && e.Node == p.call {
			scoped = e.Scoped
		}
	}
	elem, _, _ := c.r.FixedElem(recv)
	result := c.vars.resolve(c.info.Types[p.call])
	*p.edges = append(*p.edges,
		EffectEdge{Kind: EdgeAlloc, File: c.f, Node: p.call, Type: elem, Why: whyDirectCross, Scoped: scoped},
		EffectEdge{Kind: EdgeAlloc, File: c.f, Node: p.call, Type: result, Why: whyBuild, Scoped: scoped})
}
