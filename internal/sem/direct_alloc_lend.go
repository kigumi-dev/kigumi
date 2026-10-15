package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

// Whether the body only reads the parameter is mir's decision.
func (r *Result) LendableParamSlot(fn, p EntityID) bool {
	return r.Entities[fn].Kind == EntFn && r.Fn(fn).Body != 0 && r.DirectParamSlot(fn, p)
}

func (r *Result) lendableRefParam(fn, p EntityID) bool {
	if fn == 0 || p == 0 || r.Entities[fn].Kind != EntFn {
		return false
	}
	t := r.Entities[p].Type
	tt := r.Types
	return tt.Kind(t) == KRef && !tt.RefMut(t) && r.DirectAgg(tt.Node(t).Elem) && r.LendableParamSlot(fn, p)
}

func (c *checker) retagLendCross(from int, a syntax.NodeID, param EntityID, async bool) {
	if async || param == 0 || !c.r.lendableRefParam(c.r.Entities[param].Parent, param) {
		return
	}
	inner := c.t.Unparen(a)
	switch {
	case c.t.Kind(inner) == syntax.BorrowExpr && c.t.View(inner).AsBorrow().Flags()&syntax.FlagMut == 0:
		inner = c.t.View(inner).AsBorrow().Operand().ID()
	case c.borrowCoerced(a):
	default:
		return
	}
	c.retagCross(from, a, c.t.Unparen(a), inner)
}

func (c *checker) retagLendOperand(from int, op, rhs syntax.NodeID) {
	callee := c.info.Calls[op].Callee
	if callee == 0 || c.r.Entities[callee].Kind != EntFn {
		return
	}
	if params := c.r.Fn(callee).Params; len(params) == 1 && c.r.lendableRefParam(callee, params[0]) {
		c.retagCross(from, rhs, rhs)
	}
}

func (c *checker) retagCross(from int, nodes ...syntax.NodeID) {
	for i := from; i < len(*c.edges); i++ {
		e := &(*c.edges)[i]
		if e.Kind != EdgeAlloc || e.Why != whyDirectCross || !slices.Contains(nodes, e.Node) {
			continue
		}
		e.Why = whyLendCross
		if c.r.lendSites == nil {
			c.r.lendSites = map[lendSite]bool{}
		}
		for _, n := range nodes {
			c.r.lendSites[lendSite{c.f, n}] = true
		}
	}
}

type lendSite struct {
	file FileID
	node syntax.NodeID
}

func (r *Result) LendSite(file FileID, node syntax.NodeID) bool {
	return r.lendSites[lendSite{file, node}]
}

func (c *checker) borrowCoerced(a syntax.NodeID) bool {
	for _, s := range c.info.Coerce[a].Steps {
		if s.Kind == CoBorrow && !c.r.Types.RefMut(c.vars.resolve(s.To)) {
			return true
		}
	}
	return false
}
