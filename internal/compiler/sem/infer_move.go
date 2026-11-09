package sem

import "kigumi/internal/compiler/syntax"

// Matches on the declaring package since langItems has no entry for Shared
// (only effects_drop.go's dead check reads one).
func (c *checker) isSharedEnt(ent EntityID) bool {
	e := &c.r.Entities[ent]
	return e.Name == "Shared" && c.r.Packages[e.Pkg].Path == "std/alloc"
}

// Forces an owned read of a field path that would otherwise bind as a borrow,
// so the declared type comes back, not `&T`.
func (c *checker) synthMove(n syntax.NodeID, want TypeID) TypeID {
	target := c.t.Unparen(c.t.View(n).AsMove().Operand().ID())
	if c.t.Kind(target) == syntax.Lambda {
		return c.synthMovedLambda(target, want)
	}
	isField := c.t.Kind(target) == syntax.MemberExpr
	var baseType TypeID
	if isField {
		base := c.t.View(target).AsMember().Base().ID()
		baseType = c.readThrough(c.vars.resolve(c.synth(base)))
		if bn := c.r.Types.Node(baseType); bn.Kind == KNamed && c.isSharedEnt(bn.Ent) {
			c.errAt(target, cMoveShared, baseType)
			return TyPoison
		}
	}
	t := c.synthPlace(target)
	if t == TyPoison {
		return TyPoison
	}
	c.checkMoveOut(target, t, baseType, isField)
	return t
}

func (c *checker) checkMoveOut(target syntax.NodeID, t, baseType TypeID, isField bool) bool {
	if c.indexRooted(target) {
		help := "take the element out with `.takeAt` instead"
		if !c.takeAtAllowed(target) {
			help = "borrow it with `&` instead"
		}
		c.errAtHelp(target, c.noCloneHelp(t, help), cMoveIndex)
		return false
	}
	p, ok := c.buildPlace(target)
	if c.pathMode(target) == pathRef || ok && p.Deref {
		c.errAt(target, cMoveOutOfBorrow, t)
		return false
	}
	if isField && c.r.Entities[c.info.Uses[target]].Kind == EntType {
		// `.value` moves its base's storage, so the base is the place checked.
		base := c.t.Unparen(c.t.View(target).AsMember().Base().ID())
		return c.checkMoveOut(base, c.vars.resolve(c.info.Types[base]), c.memberBaseType(base), c.t.Kind(base) == syntax.MemberExpr)
	}
	if !isField {
		return true
	}
	if root, ok := c.capturedFieldRoot(target); ok {
		c.errAt(target, cMoveCapturedField, c.placeName(target), c.r.Entities[root].Name)
		return false
	}
	if c.customDrop(baseType) && !c.dropSelf(c.t.View(target).AsMember().Base().ID()) {
		name := c.takeName(target)
		c.errAt(target, cMoveDropType, name, baseType, name)
		return false
	}
	if isField && !c.r.IsCopy(t) && c.dropSelf(c.t.View(target).AsMember().Base().ID()) {
		if c.info.FieldMoves == nil {
			c.info.FieldMoves = map[syntax.NodeID]bool{}
		}
		c.info.FieldMoves[target] = true
	}
	return !c.rejectTake(target, t, takeMoved)
}

// memberBaseType is the type a member's base reads as, seen through a `Shared[T]`.
func (c *checker) memberBaseType(m syntax.NodeID) TypeID {
	if c.t.Kind(m) != syntax.MemberExpr {
		return 0
	}
	return c.readThrough(c.vars.resolve(c.info.Types[c.t.View(m).AsMember().Base().ID()]))
}

func (c *checker) customDrop(t TypeID) bool {
	bn := c.r.Types.Node(t)
	return bn.Kind == KNamed && c.r.typeDecl(bn.Ent).Drop != 0
}

// dropSelf: n is the `self` of the `drop` being checked, whose own fields that drop takes apart.
func (c *checker) dropSelf(n syntax.NodeID) bool {
	n = c.t.Unparen(n)
	if c.t.Kind(n) != syntax.Ident || c.fn == 0 || c.r.Entities[c.fn].Kind != EntFn {
		return false
	}
	info := c.r.Fn(c.fn)
	if info.Owner == 0 || c.info.Uses[n] != info.SelfParam {
		return false
	}
	t := c.r.Types.Node(c.vars.resolve(c.info.Types[n]))
	return t.Kind == KNamed && t.Ent == info.Owner && (c.r.typeDecl(info.Owner).Drop == c.fn || info.Recv == RecvMove)
}

func (c *checker) capturedFieldRoot(field syntax.NodeID) (EntityID, bool) {
	ent := c.fieldRoot(field)
	return ent, ent != 0 && !lentReceiver(&c.r.Entities[ent]) && c.capturedHere(ent)
}

func (c *checker) fieldRoot(field syntax.NodeID) EntityID {
	for b := field; ; {
		switch c.t.Kind(b) {
		case syntax.Paren:
			b = c.t.View(b).AsParen().Inner().ID()
		case syntax.MemberExpr:
			b = c.t.View(b).Base().ID()
		case syntax.Ident:
			return c.info.Uses[b]
		default:
			return 0
		}
	}
}

// synthMovedLambda: `move` before a closure literal takes the move-only captures it only reads
// instead of sharing them, so whoever owns the closure drops them.
func (c *checker) synthMovedLambda(n syntax.NodeID, want TypeID) TypeID {
	var t TypeID
	if want != 0 {
		t = c.setType(n, c.synthKind(n, want))
	} else {
		t = c.synth(n)
	}
	if c.r.Types.Kind(t) != KClosure {
		return t
	}
	info := c.r.closure(c.r.Types.Node(t).Ent)
	for i := range info.Captures {
		if info.Captures[i].Mode == capRead {
			info.Captures[i].Mode = capMove
		}
	}
	return t
}

// lambdaArg is a lambda argument, written plain or as `move <lambda>`; both wait for the other arguments to fix their parameter types.
func (c *checker) lambdaArg(a syntax.NodeID) bool {
	if c.t.Kind(a) == syntax.MoveExpr {
		a = c.t.Unparen(c.t.View(a).AsMove().Operand().ID())
	}
	return c.t.Kind(a) == syntax.Lambda
}
