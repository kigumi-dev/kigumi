package sem

import "kigumi/internal/syntax"

// The edge stays only while some body boxes the parameter.
const whyBoxedArg = 23

type boxedForward struct{ from, to EntityID }

// paramRead names the parameter n reads when its value arrives in a box the caller made.
func (c *checker) paramRead(n syntax.NodeID) EntityID {
	n = c.t.Unparen(n)
	if c.t.Kind(n) != syntax.Ident {
		return 0
	}
	ent := c.info.Uses[n]
	if ent == 0 || c.r.Entities[ent].Kind != EntParam || c.r.Entities[ent].Flags&(EfMut|EfCapturedMut|EfAddrTaken|EfSelf) != 0 {
		return 0
	}
	owner := c.r.Entities[ent].Parent
	switch c.r.Entities[owner].Kind {
	case EntClosure:
		return ent
	case EntFn:
		if owner == c.fn && len(c.lambdas) == 0 && c.r.boxesItsParams(owner) {
			return ent
		}
	}
	return 0
}

// Methods, exports and async bodies are entered some other way than a plain call.
func (r *Result) boxesItsParams(fn EntityID) bool {
	info := r.Fn(fn)
	return info.Owner == 0 && info.Recv == RecvNone && info.Export == "" && info.Abi == "" && info.Declared&EffAsync == 0
}

func (c *checker) noteBoxedParam(ent EntityID) {
	if c.r.Entities[c.r.Entities[ent].Parent].Kind != EntFn {
		return
	}
	if c.r.boxedParams == nil {
		c.r.boxedParams = map[EntityID]bool{}
	}
	c.r.boxedParams[ent] = true
}

func (c *checker) boxedArg(a syntax.NodeID, p EntityID) {
	if p == 0 || c.r.Entities[p].Kind != EntParam || c.constScalar(a) {
		return
	}
	fn := c.r.Entities[p].Parent
	if c.r.Entities[fn].Kind != EntFn || !c.r.boxesItsParams(fn) || !c.r.ScalarBoxes(c.r.Entities[p].Type) {
		return
	}
	t := c.argType(a)
	if c.r.Types.ContainsVar(t) || !c.r.ScalarBoxes(t) {
		return
	}
	if src := c.paramRead(a); src != 0 {
		if c.r.Entities[c.r.Entities[src].Parent].Kind == EntFn {
			c.r.boxedForwards = append(c.r.boxedForwards, boxedForward{src, p})
		}
		return
	}
	if c.readsBox(a) {
		return
	}
	c.edge(EffectEdge{Kind: EdgeAlloc, Node: a, Type: t, Why: whyBoxedArg, Param: p, Captured: c.capturedRead(a)})
}

func (r *Result) settleBoxedParams() {
	for changed := true; changed; {
		changed = false
		for _, f := range r.boxedForwards {
			if r.boxedParams[f.to] && !r.boxedParams[f.from] {
				r.boxedParams[f.from], changed = true, true
			}
		}
	}
}
