package sem

import "kigumi/internal/syntax"

func (c *checker) assignStmt(n syntax.NodeID) {
	node := c.t.Nodes[n]
	lhs, rhs := syntax.NodeID(node.Lhs), syntax.NodeID(node.Rhs)
	op := c.t.Toks[node.Tok].Kind
	if c.t.Kind(lhs) == syntax.Ident && c.t.TokText(c.t.Nodes[lhs].Tok) == "_" && !op.IsCompoundAssign() {
		t := c.synth(rhs)
		c.edge(EffectEdge{Kind: EdgeDrop, Type: t, Node: rhs})
		c.consumeWhole(rhs)
		return
	}
	target := c.synthPlace(lhs)
	if staticEnt, ok := c.staticRootOf(lhs); ok {
		c.checkStaticAssign(lhs, staticEnt)
	} else if c.indexAssignUnsupported(lhs) {
		c.errAt(lhs, cIndexAssignUnsupported)
	} else if !c.isMutablePlace(lhs) {
		c.reportImmutable(lhs)
	} else if root := c.rootOf(lhs); root != 0 {
		c.noteCaptureWrite(lhs, root)
	}
	c.crossStore(lhs, target)
	c.invalidatePlace(lhs)
	c.rejectAsyncSelfAssign(lhs)
	c.noteWholeSelfAssign(lhs)
	if op.IsCompoundAssign() {
		c.compoundAssign(n, lhs, rhs, target)
		return
	}
	c.noteWholeRefAssign(lhs)
	if elem, ok := c.r.MutRefElem(target); ok && c.isIdentPlace(lhs) {
		target = elem
	}
	c.check(rhs, target)
	c.applyFieldWrite(lhs, rhs)
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
