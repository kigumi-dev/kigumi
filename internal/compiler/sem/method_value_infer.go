package sem

func (c *checker) markDeferrable(inst []TypeID, shape TypeID) {
	if c.methodValues.depth == 0 {
		return
	}
	for _, t := range inst {
		if i, ok := c.vars.index(c.vars.resolve(t)); ok {
			if c.methodValues.deferrable == nil {
				c.methodValues.deferrable = map[uint32]string{}
			}
			c.methodValues.deferrable[i] = c.r.TypeString(shape)
		}
	}
}

func (c *checker) deferInference(i uint32) bool {
	if _, ok := c.methodValues.deferrable[i]; !ok || c.methodValues.depth == 0 {
		return false
	}
	c.methodValues.pending = append(c.methodValues.pending, pendingInfer{v: i, depth: c.methodValues.depth})
	return true
}

func (c *checker) keepUnsettled() {
	if len(c.methodValues.pending) == 0 {
		return
	}
	tt := c.r.Types
	for _, n := range c.touched {
		open := tt.ContainsVar(c.info.Types[n])
		if def := c.info.Defs[n]; def != 0 {
			open = open || tt.ContainsVar(c.r.Entities[def].Type)
		}
		if co, ok := c.info.Coerce[n]; ok {
			open = open || tt.ContainsVar(co.From) || len(co.Steps) > 0 && tt.ContainsVar(co.Steps[len(co.Steps)-1].To)
		}
		for _, t := range c.info.Calls[n].Inst {
			open = open || tt.ContainsVar(t)
		}
		if open {
			c.methodValues.nodes = append(c.methodValues.nodes, n)
		}
	}
	for _, id := range c.closures {
		if info := c.r.closure(id); tt.ContainsVar(info.Sig) {
			c.methodValues.closures = append(c.methodValues.closures, id)
		}
	}
}

func (c *checker) openBlock() { c.methodValues.depth++ }

func (c *checker) closeBlock() {
	mv := &c.methodValues
	depth := mv.depth
	mv.depth--
	if len(mv.pending) == 0 {
		return
	}
	kept := mv.pending[:0]
	for i, p := range mv.pending {
		if p.depth < depth {
			kept = append(kept, p)
			continue
		}
		open, ok := c.vars.index(c.vars.resolve(c.r.Types.Var(p.v)))
		if !ok {
			continue
		}
		if outer := c.outerPending(mv.pending[:i], open, depth); outer != 0 {
			kept = append(kept, pendingInfer{v: p.v, depth: outer})
			continue
		}
		help := "a call with arguments in this block, or pass it to a parameter bound like `F: Fn[" + mv.deferrable[p.v] + "]`"
		c.errAt(c.vars.origin[p.v], cInferUnresolved, c.vars.names[p.v], help)
		c.vars.setBinding(open, TyPoison)
	}
	mv.pending = kept
	c.resolveClosures(mv.closures)
	c.resolveTouched(mv.nodes)
	if len(mv.pending) == 0 {
		mv.closures, mv.nodes = mv.closures[:0], mv.nodes[:0]
	}
}

func (c *checker) outerPending(earlier []pendingInfer, open uint32, depth int) int {
	for _, q := range earlier {
		if q.depth >= depth {
			continue
		}
		if i, ok := c.vars.index(c.vars.resolve(c.r.Types.Var(q.v))); ok && i == open {
			return q.depth
		}
	}
	return 0
}
