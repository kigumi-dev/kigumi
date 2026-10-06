package sem

import (
	"kigumi/internal/diag"
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

// stmt checks one statement and returns its type.
func (c *checker) stmt(n syntax.NodeID) TypeID {
	node := c.t.Nodes[n]
	var t TypeID
	switch node.Kind {
	case syntax.LetStmt:
		c.letStmt(n)
		t = TyUnit
	case syntax.AssignStmt:
		c.assignStmt(n)
		t = TyUnit
	case syntax.ExprStmt:
		t = c.exprStmt(n)
	case syntax.DeferStmt, syntax.ErrdeferStmt:
		c.cleanupStmt(n)
		t = TyUnit
	default:
		t = c.synth(n)
	}
	c.finish()
	c.setType(n, t)
	return t
}

func (c *checker) exprStmt(n syntax.NodeID) TypeID {
	e := syntax.NodeID(c.t.Nodes[n].Lhs)
	c.stmtPos = e
	t := c.vars.resolve(c.synth(e))
	if c.r.Types.Kind(t) == KUntyped {
		t = c.adopt(c.literalNode(e), defaultOf(t))
	}
	if _, _, ok := c.r.Types.IsResult(t); ok {
		start := c.t.Span(e).Start
		c.errFix(e, diag.Fix{Title: "Discard with `_ =`", Loc: diag.At(c.t.File, token.Span{Start: start, End: start}), NewText: "_ = "}, cUnusedResult, t)
		return TyPoison
	}
	if t == TyNever {
		return TyNever
	}
	if t != TyUnit && t != TyPoison {
		c.edge(EffectEdge{Kind: EdgeDrop, Type: t, Node: e})
		c.errAt(e, cUnusedValue, t)
	}
	return t
}

func (c *checker) letStmt(n syntax.NodeID) {
	s := letStmt(c.t, n)
	var annotated TypeID
	if s.Type != 0 {
		annotated = c.r.resolveType(c.f, c.scope, s.Type, posLocal)
	}
	var got TypeID
	if annotated != 0 {
		got = c.checkAlias(s.Init, annotated, c.f, s.Type)
	} else {
		got = c.synth(s.Init)
		if c.r.Types.Kind(c.vars.resolve(got)) == KUntyped {
			got = c.adopt(c.literalNode(s.Init), defaultOf(c.vars.resolve(got)))
		}
	}
	if s.Else != 0 {
		c.letElse(n, s, got)
		return
	}
	mut := s.Flags&syntax.FlagMut != 0
	if c.t.Kind(s.Pattern) == syntax.PatBind {
		typ := got
		borrow := false
		switch {
		case annotated != 0:
			typ = annotated
		case c.borrowedPlace(s.Init) && !c.r.isCopy(c.vars.resolve(got)) && c.r.Types.Kind(c.vars.resolve(got)) != KRef:
			typ, borrow = c.r.Types.Ref(c.vars.resolve(got), false), true
		}
		c.info.Pats[s.Pattern] = PatInfo{Kind: PatBinding, Type: got, Borrow: borrow}
		c.declareLocal(s.Pattern, c.t.TokText(c.t.Nodes[s.Pattern].Tok), typ, mut)
		c.setType(s.Pattern, typ)
		// `let g = f` keeps a callback parameter's polymorphism: calls of g
		// count as calls of f.
		if p := c.polyParam(s.Init); p != 0 && !mut {
			c.paramAlias[c.info.Defs[s.Pattern]] = p
		}
		if !mut {
			if c.letInit == nil {
				c.letInit = map[EntityID]syntax.NodeID{}
			}
			c.letInit[c.info.Defs[s.Pattern]] = s.Init
			if w, ok := c.info.CoerceWitness[s.Init]; ok {
				c.r.localWitness[c.info.Defs[s.Pattern]] = w
			}
			c.trackKnownFn(s.Init, c.info.Defs[s.Pattern])
		}
		// Unlike paramAlias this survives `mut`: the local can still hold the restricted value.
		if p := c.restrictedAliasOf(s.Init); p != 0 {
			c.escapeAlias[c.info.Defs[s.Pattern]] = p
		}
		if c.t.Kind(s.Init) == syntax.RecordLit {
			c.recordEffectFacts(s.Init, c.r.identPlace(c.info.Defs[s.Pattern]))
		}
		c.consumeWhole(s.Init)
		return
	}
	if c.t.Kind(s.Pattern) == syntax.PatWildcard {
		c.setType(s.Pattern, got)
		c.edge(EffectEdge{Kind: EdgeDrop, Type: got, Node: s.Init})
		c.consumeWhole(s.Init)
		return
	}
	pi := c.checkPatternOn(s.Pattern, s.Init, got, mut, c.scope)
	c.taintPatternBindings(s.Pattern, c.restrictedAliasOf(s.Init))
	if !c.irrefutable(s.Pattern, got) {
		c.errAt(s.Pattern, cPatternRefutable, c.witnessText(s.Pattern, got))
	}
	_ = pi
}

// letElse handles `let pat = e else { ... }`: bindings live after the
// statement and the else block must diverge.
func (c *checker) letElse(n syntax.NodeID, s letSlots, got TypeID) {
	saved := c.scope
	elseType := c.check(s.Else, 0)
	if c.vars.resolve(elseType) != TyNever {
		c.errAt(s.Else, cLetElseMustDiverge)
	}
	c.scope = saved
	c.checkPatternOn(s.Pattern, s.Init, got, s.Flags&syntax.FlagMut != 0, c.scope)
	c.taintPatternBindings(s.Pattern, c.restrictedAliasOf(s.Init))
	if c.irrefutable(s.Pattern, got) {
		c.errAt(s.Pattern, cLetElseIrrefutable)
	}
	pos, _ := c.patternFacts(s.Pattern, c.factPlace(s.Init))
	c.addFacts(pos)
	c.info.Matches[n] = MatchInfo{Exhaustive: false}
}

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
	c.invalidatePlace(lhs)
	c.rejectAsyncSelfAssign(lhs)
	c.invalidateFieldWitness(lhs)
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

// cleanupStmt checks a defer/errdefer body as a Unit statement with
// control flow forbidden inside.
