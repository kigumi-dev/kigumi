package sem

import "kigumi/internal/compiler/syntax"

// coerce makes the value of node n (of type got) fit want at a coercion
// site; it returns want, or TyPoison after a mismatch.
func (c *checker) coerce(n syntax.NodeID, got, want TypeID) TypeID {
	if want == 0 {
		return got
	}
	steps, ok := c.coerceSteps(n, got, want)
	if !ok {
		c.mismatch(n, want, got)
		return TyPoison
	}
	if len(steps) > 0 {
		c.info.Coerce[n] = Coercion{From: got, Steps: steps}
		c.touched = append(c.touched, n)
		c.chargeCoercion(n, c.info.Coerce[n])
		if steps[0].Kind == CoLiteral {
			c.adoptTails(c.literalNode(n), steps[0].To)
			c.setType(c.literalNode(n), steps[0].To)
		}
	}
	return want
}

func (c *checker) coerceSteps(n syntax.NodeID, got, want TypeID) ([]CoStep, bool) {
	tt := c.r.Types
	got, want = c.vars.resolve(got), c.vars.resolve(want)
	if tt.Kind(got) == KFn && tt.Kind(want) == KFn {
		if item, ok := c.genericFnItemCallable(n); ok {
			return nil, c.coerceGenericFnItem(n, item, got, want)
		}
	}
	switch {
	case got == TyPoison || want == TyPoison:
		return nil, true
	case got == TyNever:
		return []CoStep{{CoNever, want}}, true
	case tt.Kind(got) == KUntyped && tt.Kind(want) == KVar:
		if c.vars.unify(got, want) {
			if i, ok := c.vars.index(want); ok {
				c.vars.notePending(i, c.literalNode(n))
			}
			return nil, true
		}
	case c.vars.unifiable(got, want):
		c.vars.unify(got, want)
		return nil, true
	case tt.Kind(got) == KUntyped && tt.IsNumeric(want):
		if c.fitsQuiet(c.literalNode(n), want) {
			return []CoStep{{CoLiteral, want}}, true
		}
		return nil, false
	case c.coerceErasedValue(n, got, want):
		return nil, true
	}
	if elem, ok := tt.IsOption(want); ok {
		if s, ok := c.coerceSteps(n, got, elem); ok {
			return append(s, CoStep{CoSome, want}), true
		}
	}
	if val, _, ok := tt.IsResult(want); ok {
		if _, _, gotIsResult := tt.IsResult(got); !gotIsResult {
			if s, ok := c.coerceSteps(n, got, val); ok {
				return append(s, CoStep{CoOk, want}), true
			}
		}
	}
	if tt.Kind(got) == KUntyped {
		steps, ok := c.coerceSteps(n, defaultOf(got), want)
		if ok && len(steps) > 0 && steps[0].Kind != CoLiteral {
			steps = append([]CoStep{{CoLiteral, defaultOf(got)}}, steps...)
		}
		return steps, ok
	}
	if w := tt.Node(want); w.Kind == KRef && w.Flags&flagMut == 0 && tt.Kind(got) != KRef && c.r.isCopy(got) && c.vars.unifiable(got, w.Elem) {
		c.vars.unify(got, w.Elem)
		return []CoStep{{CoBorrow, want}}, true
	}
	if steps, ok := c.lendExistential(n, got, want); ok {
		return steps, true
	}
	if tt.Kind(want) == KIface && c.r.objectSafe(tt.Node(want).Ent) {
		if res := c.r.conforms(got, want, c.pkg); res.ok {
			c.implContracts(n, got, want, slotRoot)
			for _, w := range res.witnesses {
				if len(c.r.Fn(w).Witnesses) > 0 {
					c.errAt(n, cIfaceBoxWitness, got, want, c.r.Entities[w].Name)
					return []CoStep{{CoExistential, want}}, true
				}
				c.recordCtxValueRef(n, w)
			}
			c.info.CoerceWitness[n] = res.witnesses
			c.edge(EffectEdge{Kind: EdgeAlloc, Type: want, From: got, Node: n})
			return []CoStep{{CoExistential, want}}, true
		}
	}
	if tt.Kind(want) == KFn {
		return c.coerceToFn(n, got, want)
	}
	return nil, false
}

// coerceToFn erases a fn item or fn value into want; shapes must match and contracts be at least as strong.
func (c *checker) coerceToFn(n syntax.NodeID, got, want TypeID) ([]CoStep, bool) {
	tt := c.r.Types
	g, w := tt.Node(got), tt.Node(want)
	if w.Flags&fnCAbi != 0 {
		return c.coerceToCFn(n, got, want)
	}
	if g.Kind == KClosure {
		return c.closureErase(n, g.Ent, want)
	}
	if g.Kind != KFn || g.Flags&fnCAbi != 0 {
		return nil, false
	}
	if (g.Flags^w.Flags)&uint16(EffUnsafe) != 0 || !c.fitsErased(got, want, true) {
		return nil, false
	}
	c.refScopeContract(n, got, want)
	if i, ok := c.firstEscapeMismatch(got, want); ok {
		c.errAt(n, cEscapingCoerce, c.typeText(got), i+1, c.typeText(want))
		return nil, true
	}
	need := Effects(w.Flags) & (EffPure | EffNoalloc)
	if Effects(g.Flags)&need != need {
		if item := c.info.Uses[n]; item != 0 && c.r.Entities[item].Kind == EntFn {
			c.edge(EffectEdge{Kind: EdgeContract, Target: item, Mods: need, Node: n})
		} else {
			c.errAt(n, cFnValueEffects, got, want, effectsText(need&^Effects(g.Flags)))
			return nil, true
		}
	}
	if item := c.info.Uses[n]; item != 0 && c.r.Entities[item].Kind == EntFn {
		c.noteNoallocErase(n, want)
		return []CoStep{{CoFnItem, want}}, true
	}
	return nil, true
}
