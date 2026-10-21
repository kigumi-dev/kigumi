package sem

import (
	"maps"

	"kigumi/internal/syntax"
)

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
	once := c.onceLoopBegin()
	switch {
	case s.Pattern != 0:
		iter := c.vars.resolve(c.synthPlace(s.Head))
		elem := c.iterElement(n, s.Head, iter)
		if !c.r.isCopy(c.vars.resolve(elem)) {
			c.rejectFieldLeaf(c.t.Unparen(s.Head))
			c.rejectForOfBorrowed(s.Head, iter, elem)
		}
		c.reserveUse(s.Head)
		c.checkPattern(s.Pattern, elem, false, c.scope)
		c.trackLoopPlaces(s.Pattern, s.Head)
		c.noteLoopAlias(s.Pattern, s.Head)
		c.taintPatternBindings(s.Pattern, c.restrictedAliasOf(s.Head))
		once.entry = maps.Clone(c.once.called)
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
	c.onceLoopBody(once)
	c.onceLoopEnd(once, c.check(s.Body, TyUnit), bare)
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
		c.noteLoopValues(n, mine.nodes)
		return c.merge(mine.nodes, mine.types)
	}
	if forever {
		return TyNever
	}
	return TyUnit
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
	c.onceLeave(kw == "break")
	return TyNever
}

func (c *checker) noteLoopValues(loop syntax.NodeID, nodes []syntax.NodeID) {
	if c.loopValues == nil {
		c.loopValues = map[syntax.NodeID][]syntax.NodeID{}
	}
	c.loopValues[loop] = nodes
}

func (c *checker) isRangeType(at syntax.NodeID, t TypeID) bool {
	node := c.r.Types.Node(t)
	return node.Kind == KNamed && node.Ent != 0 && node.Ent == c.r.langItem(c.f, at, "Range")
}
