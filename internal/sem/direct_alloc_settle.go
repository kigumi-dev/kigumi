package sem

import "kigumi/internal/syntax"

type pendingAlloc struct {
	edges *[]EffectEdge
	at    int
	keep  TypeID
}

func (c *checker) buildEdge(n syntax.NodeID, t TypeID) {
	t = c.vars.resolve(t)
	c.edge(EffectEdge{Kind: EdgeAlloc, Node: n, Type: t, Why: whyBuild})
	if c.r.Types.ContainsVar(t) {
		c.allocPending = append(c.allocPending, pendingAlloc{edges: c.edges, at: len(*c.edges) - 1})
	}
}

func (c *checker) chargeCoercion(n syntax.NodeID, co Coercion) {
	fixed := c.fixedValue(n, co.From)
	for _, s := range co.Steps {
		switch {
		case (s.Kind == CoSome || s.Kind == CoOk) && !fixed:
			c.buildEdge(n, s.To)
		case s.Kind == CoBorrow:
			c.crossBorrow(n)
		}
	}
}

func (c *checker) fixedPayload(args []syntax.NodeID) bool {
	for _, a := range args {
		if !c.fixedValue(a, c.info.Types[a]) {
			return false
		}
	}
	return true
}

func (c *checker) fixedValue(n syntax.NodeID, t TypeID) bool {
	for c.t.Kind(n) == syntax.Paren {
		n = syntax.NodeID(c.t.Nodes[n].Lhs)
	}
	if v, ok := c.info.Literals[n]; ok {
		return v.Kind == constInt || v.Kind == constFloat || v.Kind == constString || v.Kind == constChar
	}
	switch c.t.Kind(n) {
	case syntax.UnitLit, syntax.CharLit:
		return true
	case syntax.StringLit:
		for _, p := range c.t.StringParts(n) {
			if p.Expr != 0 {
				return false
			}
		}
		return true
	case syntax.Block, syntax.CallExpr:
		return c.vars.resolve(t) == TyUnit
	}
	return c.nullaryValue(n)
}

func (c *checker) settleAllocs() {
	kept := c.allocPending[:0]
	for _, p := range c.allocPending {
		e := &(*p.edges)[p.at]
		t := c.vars.resolve(e.Type)
		switch {
		case c.r.Types.ContainsVar(t):
			kept = append(kept, p)
		case e.Why == whyDirectCross && (t == p.keep || !c.r.DirectAgg(t) && !c.r.containsClosure(t)):
			t = 0
		}
		e.Type = t
	}
	c.allocPending = kept
}
