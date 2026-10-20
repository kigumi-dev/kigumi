package sem

import "kigumi/internal/syntax"

// loopBreaks is nil outside a bare `for { }` loop, where a valued break is an error.
type loopBreaks struct {
	nodes []syntax.NodeID
	types []TypeID
}

func (c *checker) synthFor(n syntax.NodeID) TypeID {
	s := forExpr(c.t, n)
	c.loopSpans = append(c.loopSpans, c.t.Span(n))
	bare := s.Pattern == 0 && s.Head == 0
	saved := c.scope
	c.pushScope(ScopeArm, n)
	switch {
	case s.Pattern != 0:
		iter := c.vars.resolve(c.synthPlace(s.Head))
		elem := c.iterElement(n, s.Head, iter)
		c.checkPattern(s.Pattern, elem, false, c.scope)
		c.trackLoopPlaces(s.Pattern, s.Head)
		c.noteLoopAlias(s.Pattern, s.Head)
		c.taintPatternBindings(s.Pattern, c.restrictedAliasOf(s.Head))
		if !c.irrefutable(s.Pattern, elem) {
			c.errAt(s.Pattern, cPatternRefutable, c.witnessText(s.Pattern, elem))
		}
		c.rejectLoopScalarMutBinds(s.Pattern)
	case s.Head != 0:
		c.condition(s.Head)
	}
	c.dropUnstableFacts()
	entry := c.snapshotFacts()
	broke := c.brokeOut
	c.brokeOut = false
	savedBreaks := c.breaks
	var mine *loopBreaks
	if bare {
		mine = &loopBreaks{}
	}
	c.breaks = mine
	c.loop++
	c.check(s.Body, TyUnit)
	c.loop--
	c.breaks = savedBreaks
	c.popScope(saved)
	c.facts = entry
	forever := bare && !c.brokeOut
	c.brokeOut = broke
	if !bare {
		if forever {
			return TyNever
		}
		return TyUnit
	}
	if len(mine.types) > 0 {
		return c.merge(mine.nodes, mine.types)
	}
	if forever {
		return TyNever
	}
	return TyUnit
}

func (c *checker) iterElement(loop, n syntax.NodeID, iter TypeID) TypeID {
	tt := c.r.Types
	if iter == TyPoison {
		return TyPoison
	}
	if tt.Kind(iter) == KUntyped {
		iter = c.adopt(c.literalNode(n), defaultOf(iter))
	}
	// Element keeps the head's borrow, as &T/&mut T bind an argument.
	if tt.Kind(iter) == KRef {
		mut := tt.Node(iter).Flags&flagMut != 0
		elem := c.iterElement(loop, n, c.vars.resolve(tt.Node(iter).Elem))
		if elem == TyPoison {
			return TyPoison
		}
		return tt.Ref(elem, mut)
	}
	if iter == TyString {
		c.errAt(n, cForString)
		return TyPoison
	}
	if iter == TyBytes {
		c.info.Calls[loop] = CallInfo{Kind: CallIter}
		return TyU8
	}
	node := tt.Node(iter)
	if node.Kind == KNamed && (node.Ent == tt.arrayEnt || c.r.isFixedArray(iter)) {
		c.info.Calls[loop] = CallInfo{Kind: CallIter}
		return node.Args[0]
	}
	if rng := c.r.langItems["Range"]; node.Kind == KNamed && rng != 0 && node.Ent == rng {
		c.info.Calls[loop] = CallInfo{Kind: CallIter}
		return node.Args[0]
	}
	if me := c.r.mapEntity(); node.Kind == KNamed && me != 0 && node.Ent == me {
		tup, _ := tt.tupleEntity(2)
		c.info.Calls[loop] = CallInfo{Kind: CallIterMap, Callee: me}
		return tt.Named(tup, []TypeID{node.Args[0], node.Args[1]})
	}
	c.errAt(n, cNotIterable, iter)
	return TyPoison
}

// Rejects &mut-of-scalar for-binds: the vm/native backends give such an element
// no cell to write through, unlike an ordinary &mut local.
func (c *checker) rejectLoopScalarMutBinds(p syntax.NodeID) {
	if c.t.Kind(p) == syntax.PatBind {
		if ent := c.info.Defs[p]; ent != 0 {
			if elem, ok := c.r.ScalarRefElem(c.r.Entities[ent].Type); ok {
				c.errAt(p, cForLoopScalarMut, elem)
			}
		}
	}
	for ch := range c.t.View(p).Children() {
		c.rejectLoopScalarMutBinds(ch.ID())
	}
}

// Skips the lang-item mechanism's missing-package diagnostic: most programs
// don't import std/map, so a `for` over something else shouldn't blame it.
func (r *Result) mapEntity() EntityID {
	pkg, ok := r.pathIndex["std/map"]
	if !ok {
		return 0
	}
	return r.Scopes[r.Packages[pkg].Scope].Names["Map"].Ent
}

func (c *checker) synthReturn(n syntax.NodeID) TypeID {
	ret := c.t.View(n).AsReturn()
	if c.cleanup > 0 {
		c.errAt(n, cCleanupControlFlow, "`return`")
	}
	if !ret.Value().Valid() {
		if c.retType != TyUnit && c.retType != TyPoison && c.retType != c.r.unitResult() {
			c.errAt(n, cReturnMissingValue, c.retType)
		}
		return TyNever
	}
	val := ret.Value().ID()
	c.exitCheck.returning = append(c.exitCheck.returning, exitSite{value: val, lambdas: len(c.lambdas)})
	defer func() { c.exitCheck.returning = c.exitCheck.returning[:len(c.exitCheck.returning)-1] }()
	c.check(val, c.retType)
	if len(c.lambdas) == 0 {
		c.retLeaves = append(c.retLeaves, val)
	}
	c.checkEscapeReturn(val)
	c.consumeWhole(val)
	return TyNever
}

func (c *checker) synthBreak(n syntax.NodeID) TypeID {
	kw := c.t.View(n).Tok().Text()
	if c.cleanup > 0 {
		c.errAt(n, cCleanupControlFlow, "`"+kw+"`")
	} else if c.loop == 0 {
		c.errAt(n, cBreakOutsideLoop, kw)
		return TyPoison
	}
	if kw == "break" {
		c.brokeOut = true
		val := c.t.View(n).AsBreak().Value().ID()
		if val != 0 && c.cleanup == 0 {
			c.reserveUse(val)
		}
		switch {
		case c.breaks != nil:
			t, node := TyUnit, n
			if val != 0 {
				t, node = c.synth(val), val
			}
			c.breaks.nodes = append(c.breaks.nodes, node)
			c.breaks.types = append(c.breaks.types, t)
		case val != 0:
			c.synth(val)
			c.errAt(n, cBreakValueLoopKind)
		}
	}
	return TyNever
}
