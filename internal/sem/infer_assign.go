package sem

import "kigumi/internal/syntax"

func (c *checker) assignStmt(n syntax.NodeID) {
	a := c.t.View(n).AsAssign()
	lhs, rhs := a.Target().ID(), a.Value().ID()
	op := a.Op().Kind()
	if c.t.Kind(lhs) == syntax.Ident && c.t.View(lhs).Tok().Text() == "_" && !op.IsCompoundAssign() {
		t := c.synth(rhs)
		c.edge(EffectEdge{Kind: EdgeDrop, Type: t, Node: rhs})
		c.consumeWhole(rhs)
		return
	}
	target := c.synthPlace(lhs)
	if c.rejectCtxKeyWrite(lhs, "") {
		c.check(rhs, target)
		return
	}
	c.rejectScalarCopyWrite(lhs)
	if !op.IsCompoundAssign() {
		c.reserveAssign(lhs, target)
	}
	if staticEnt, ok := c.staticRootOf(lhs); ok {
		c.checkStaticAssign(lhs, staticEnt)
	} else if c.indexAssignUnsupported(lhs) {
		c.errAt(lhs, cIndexAssignUnsupported)
	} else if !c.isMutablePlace(lhs) && op.IsCompoundAssign() {
		c.reportImmutableBase(lhs)
	} else if !c.isMutablePlace(lhs) {
		c.reportImmutable(lhs)
	} else if root := c.rootOf(lhs); root != 0 {
		c.noteCaptureWrite(lhs, root)
	}
	deferred := !op.IsCompoundAssign() && c.copyShape(lhs, rhs)
	_, refTarget := c.r.MutRefElem(c.vars.resolve(target))
	late := !deferred && !op.IsCompoundAssign() && c.nullaryShape(rhs)
	boxedSlot := !deferred && !late && c.crossStore(lhs, rhs, target) || refTarget && c.isIdentPlace(lhs)
	c.invalidatePlace(lhs)
	c.rejectAsyncSelfAssign(lhs)
	c.noteWholeSelfAssign(lhs)
	if op.IsCompoundAssign() {
		if t := c.vars.resolve(target); c.r.Types.Kind(t) == KRef && c.isIdentPlace(lhs) {
			target = c.r.Types.Node(t).Elem
		}
		c.compoundAssign(n, lhs, rhs, target)
		if boxedSlot {
			c.scalarStore(n, lhs, target, c.storeRef(lhs, refTarget))
		}
		return
	}
	c.noteWholeRefAssign(lhs)
	if elem, ok := c.r.MutRefElem(target); ok && c.isIdentPlace(lhs) {
		target = elem
	}
	c.check(rhs, target)
	c.rejectFieldValue(rhs)
	c.onceReassign(lhs)
	if late {
		boxedSlot = c.crossStore(lhs, rhs, target) || boxedSlot
	}
	if deferred {
		if c.elementCopy(lhs, rhs, target) || c.fieldCopy(lhs, rhs, target) {
			c.crossOwnedRoot(lhs)
			if src := c.stripParen(rhs); c.rootOf(src) != c.rootOf(lhs) {
				c.crossOwnedRoot(src)
			}
		} else {
			boxedSlot = c.crossStore(lhs, rhs, target) || boxedSlot
		}
	}
	if boxedSlot && !c.constScalar(rhs) {
		c.scalarStore(rhs, lhs, target, c.storeRef(lhs, refTarget))
	}
	c.applyFieldWrite(lhs, rhs)
	c.noteStrongFieldWrite(n, lhs)
	c.noteAssignKill(n, lhs, rhs)
	if c.t.Kind(lhs) != syntax.Ident {
		c.checkEscapeStore(rhs)
	} else if p := c.restrictedAliasOf(rhs); p != 0 {
		// Sticky: an earlier branch may still reach here holding the restricted parameter.
		if ent := c.info.Uses[lhs]; ent != 0 {
			c.escapeAlias[ent] = p
		}
	}
	c.consumeWhole(rhs)
	if ent, ok := c.identPlaceEnt(lhs); ok {
		c.unconsume(ent)
	}
	if c.t.Kind(rhs) == syntax.RecordLit {
		if place := c.placeOf(lhs); place != 0 {
			c.recordEffectFacts(rhs, c.r.Places[place])
		}
	}
}
