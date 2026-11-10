package sem

import "kigumi/internal/compiler/syntax"

// finish closes a full expression: every type recorded since the last
// boundary is substituted to its final, defaulted form.
func (c *checker) finish() { c.finishWith(false) }

func (c *checker) finishWith(nested bool) {
	c.bindCallableConstraints()
	c.vars.settle(c)
	c.settleOpenCaptures()
	c.settleBorrows()
	c.settleAllocs()
	c.settleInline()
	c.checkConstraints()
	c.runFFIValueChecks()
	c.runTruncateValueChecks()
	c.resolveClosures(c.closures[c.closureBase:])
	c.resolveTouched(c.touched[c.touchedBase:])
	c.keepUnsettled()
	c.vars.boundary()
	if nested {
		return
	}
	c.noteTaskUses()
	c.closures = c.closures[:c.closureBase]
	c.touched = c.touched[:c.touchedBase]
}

func (c *checker) resolveClosures(ids []EntityID) {
	for _, id := range ids {
		info := c.r.closure(id)
		info.Ret = c.vars.resolve(info.Ret)
		info.Sig = c.vars.resolve(info.Sig)
	}
}

func (c *checker) resolveTouched(nodes []syntax.NodeID) {
	var futures []taskFutureCall
	for _, n := range nodes {
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
			c.r.noteTaskConversions(c.fn, c.f, n, co)
		}
		if call, ok := c.info.Calls[n]; ok {
			for i := range call.Inst {
				call.Inst[i] = c.vars.resolve(call.Inst[i])
			}
			if call.Box != 0 {
				call.Box = c.vars.resolve(call.Box)
			}
			c.info.Calls[n] = call
			c.r.checkLifetimeEscapeType(c.f, n, c.info.Types[n], map[TypeID]bool{})
			c.r.checkTaskEscapeCall(c.f, n, call)
			c.r.noteSpawnArgs(c.f, n, call)
			futures = append(futures, c.r.taskFutureOf(c.f, n, call, c.info.Types[n])...)
		}
	}
	c.r.checkTaskFutures(c.f, futures)
}
