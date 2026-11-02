package sem

import "kigumi/internal/compiler/syntax"

// lendExistential is `&T -> &I`, `&mut T -> &mut I` and `&mut T -> &I` for a
// concrete T conforming to a dispatchable I: a view over the borrowed value.
func (c *checker) lendExistential(n syntax.NodeID, got, want TypeID) ([]CoStep, bool) {
	tt := c.r.Types
	g, w := tt.Node(got), tt.Node(want)
	if g.Kind != KRef || w.Kind != KRef || tt.Kind(w.Elem) != KIface || !c.r.objectSafe(tt.Node(w.Elem).Ent) {
		return nil, false
	}
	if w.Flags&flagMut != 0 && g.Flags&flagMut == 0 {
		return nil, false
	}
	switch tt.Kind(g.Elem) {
	case KNamed, KPrim:
	default:
		return nil, false
	}
	res := c.r.conforms(g.Elem, w.Elem, c.pkg)
	if !res.ok {
		return nil, false
	}
	c.implContracts(n, g.Elem, w.Elem, erasePos{pos: true})
	for _, wit := range res.witnesses {
		if len(c.r.Fn(wit).Witnesses) > 0 {
			c.errAt(n, cIfaceBoxWitness, g.Elem, w.Elem, c.r.Entities[wit].Name)
			return []CoStep{{CoLendExistential, want}}, true
		}
		c.recordCtxValueRef(n, wit)
	}
	c.info.CoerceWitness[n] = res.witnesses
	return []CoStep{{CoLendExistential, want}}, true
}
