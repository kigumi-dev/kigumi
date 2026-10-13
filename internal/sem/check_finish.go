package sem

// finish closes a full expression: every type recorded since the last
// boundary is substituted to its final, defaulted form.
func (c *checker) finish() {
	c.vars.settle(c)
	c.settleAllocs()
	c.settleInline()
	c.checkConstraints()
	c.runFFIValueChecks()
	for _, id := range c.closures {
		info := c.r.closure(id)
		info.Ret = c.vars.resolve(info.Ret)
		info.Sig = c.vars.resolve(info.Sig)
	}
	c.closures = c.closures[:0]
	for _, n := range c.touched {
		c.info.Types[n] = c.vars.resolve(c.info.Types[n])
		// Bindings keep the resolved type too; the backends read it from
		// the entity, not from the node.
		if def := c.info.Defs[n]; def != 0 {
			e := &c.r.Entities[def]
			e.Type = c.vars.resolve(e.Type)
			if c.r.Types.Kind(e.Type) == KUntyped {
				e.Type = defaultOf(e.Type)
			}
		}
		if co, ok := c.info.Coerce[n]; ok {
			co.From = c.vars.resolve(co.From)
			for i := range co.Steps {
				co.Steps[i].To = c.vars.resolve(co.Steps[i].To)
			}
			c.info.Coerce[n] = co
		}
		if call, ok := c.info.Calls[n]; ok {
			for i := range call.Inst {
				call.Inst[i] = c.vars.resolve(call.Inst[i])
			}
			c.info.Calls[n] = call
			c.r.checkLifetimeEscapeType(c.f, n, c.info.Types[n], map[TypeID]bool{})
		}
	}
	c.touched = c.touched[:0]
	c.vars.boundary()
}
