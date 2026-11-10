package sem

import "kigumi/internal/compiler/syntax"

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
		c.staticStoreFrees(lhs, target)
	} else if c.indexAssignUnsupported(lhs) {
		c.errAt(lhs, cIndexAssignUnsupported)
	} else if c.rejectViewAccess(lhs, accessAssign) {
	} else if !c.isMutablePlace(lhs) && op.IsCompoundAssign() {
		c.reportImmutableBase(lhs)
	} else if !c.isMutablePlace(lhs) {
		c.reportImmutable(lhs)
	} else if root := c.rootOf(lhs); root != 0 {
		c.noteCaptureWrite(lhs, root, false)
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
	} else if ent, ok := c.identPlaceEnt(lhs); ok && c.t.Kind(lhs) == syntax.Ident && !c.capturedHere(ent) {
		c.markEraseSite(rhs, siteLocal)
	}
	c.check(rhs, target)
	c.fieldContracts(lhs, rhs)
	c.rejectTempBorrowStore(rhs)
	c.storeScalarCopy(lhs, rhs, target)
	c.noteHolderAssign(lhs, rhs)
	// Only a plain local slot is a holder; a field, element or `&mut` target stores the value elsewhere (an escape).
	if ent := c.info.Uses[c.t.Unparen(lhs)]; c.t.Kind(c.t.Unparen(lhs)) == syntax.Ident &&
		c.r.Entities[ent].Kind == EntLocal && c.r.Types.Kind(c.vars.resolve(c.r.Entities[ent].Type)) != KRef {
		c.noteHolderValue(ent, rhs)
	}
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
	c.overwriteDrop(lhs, target)
	c.consumeWhole(rhs)
	if ent, ok := c.identPlaceEnt(lhs); ok {
		c.unconsume(ent)
	} else if p, ok := c.placeUnder(lhs); ok {
		c.consumed.revive(p.ent, p.fields)
	}
	if c.t.Kind(rhs) == syntax.RecordLit {
		if place := c.placeOf(lhs); place != 0 {
			c.recordEffectFacts(rhs, c.r.Places[place])
		}
	}
}
