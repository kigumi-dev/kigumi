package sem

import "kigumi/internal/compiler/syntax"

func (c *checker) fnBoundMods(fn EntityID, i int) (Effects, TypeID) {
	tt := c.r.Types
	params := tt.Node(c.r.Fn(fn).Sig).Args
	if i >= len(params) || tt.Kind(params[i]) != KParam {
		return 0, 0
	}
	shape := c.r.fnShapeOf(params[i])
	if shape == 0 {
		return 0, 0
	}
	return Effects(tt.Node(shape).Flags) & (EffPure | EffNoalloc), shape
}

func (c *checker) fnBoundContract(a syntax.NodeID, target EntityID, need Effects) {
	if need != 0 {
		c.edge(EffectEdge{Kind: EdgeContract, Target: target, Mods: need, Node: a})
	}
}

func (c *checker) fnBoundValue(a syntax.NodeID, at TypeID, bound TypeID, need Effects) {
	shape := c.r.fnShapeOf(at)
	if need == 0 || shape == 0 {
		return
	}
	if missing := need &^ Effects(c.r.Types.Node(shape).Flags); missing != 0 {
		c.errAt(a, cFnValueEffects, at, bound, effectsText(missing))
	}
}
