package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

const whyInlined = 6

type pendingInline struct {
	edges    *[]EffectEdge
	from, to int
	fn       EntityID
	call     syntax.NodeID
	base     syntax.NodeID
	args     []syntax.NodeID
}

func (r *Result) fixedInlineOp(fn EntityID) string {
	if fn == 0 || r.Entities[fn].Kind != EntFn || r.Entities[fn].Flags&EfStd == 0 {
		return ""
	}
	owner := r.Fn(fn).Owner
	if owner == 0 || owner != r.langItem(0, 0, "FixedArray") {
		return ""
	}
	switch name := r.Entities[fn].Name; name {
	case "len", "get", "at", "set", "withMut", "filled", "zeroed":
		return name
	}
	return ""
}

// FixedInline reports an op lowered in place; set and withMut also need a plain local receiver.
func (r *Result) FixedInline(fn EntityID, t TypeID) bool {
	switch r.fixedInlineOp(fn) {
	case "len":
		_, _, ok := r.fixedArrayShape(t)
		return ok
	case "get", "at", "set", "withMut", "filled":
		return r.DirectFixed(t)
	case "zeroed":
		elem, _, _ := r.fixedArrayShape(t)
		return r.DirectFixed(t) && (r.Types.IsInteger(elem) || r.Types.IsFloat(elem))
	}
	return false
}

// An async body keeps the call: its frame holds the array boxed, which the call's own summary charges.
func (c *checker) noteInline(n, base syntax.NodeID, fn EntityID, args []syntax.NodeID, from int) {
	if c.r.fixedInlineOp(fn) == "" || c.asyncBody() {
		return
	}
	c.inlinePending = append(c.inlinePending, pendingInline{edges: c.edges, from: from, to: len(*c.edges), fn: fn, call: n, base: base, args: args})
}

func (c *checker) settleInline() { c.settleInlineAt(false) }

// set and withMut wait for the body's end, as a later capture write makes the receiver a cell.
func (c *checker) settleInlineAt(bodyDone bool) {
	kept := c.inlinePending[:0]
	for _, p := range c.inlinePending {
		node := p.call
		if p.base != 0 {
			node = p.base
		}
		t := c.vars.resolve(c.info.Types[node])
		op := c.r.fixedInlineOp(p.fn)
		mut := op == "set" || op == "withMut"
		if c.r.Types.ContainsVar(t) || mut && !bodyDone || c.fixedResultOpen(p, op) {
			kept = append(kept, p)
			continue
		}
		if _, coerced := c.info.Coerce[node]; (coerced && p.base != 0) || c.r.containsClosure(t) || !c.r.FixedInline(p.fn, t) {
			continue
		}
		if mut && !c.plainLocal(p.base) {
			continue
		}
		for i := p.from; i < p.to; i++ {
			if e := &(*p.edges)[i]; inlinedEdge(e, p) {
				e.Why = whyInlined
			}
		}
		c.chargeFixedResidue(p, op, t)
	}
	c.inlinePending = kept
}

func inlinedEdge(e *EffectEdge, p pendingInline) bool {
	switch {
	case e.Kind == EdgeCall:
		return e.Target == p.fn && e.Node == p.call
	case e.Kind == EdgeAlloc && directCrossWhy(e.Why):
		return e.Node == p.call || slices.Contains(p.args, e.Node)
	}
	return false
}

// Must match MIR: a `mut self` op writes a plain local, neither a cell nor a lent receiver.
func (c *checker) plainLocal(n syntax.NodeID) bool {
	if n == 0 || c.t.Kind(n) != syntax.Ident {
		return false
	}
	ent := c.info.Uses[n]
	e := &c.r.Entities[ent]
	if e.Kind != EntLocal && e.Kind != EntParam || e.Flags&(EfCapturedMut|EfAddrTaken) != 0 || c.r.IsMutSelf(ent) {
		return false
	}
	return e.Flags&EfSelf == 0 || e.Flags&EfMove != 0
}
