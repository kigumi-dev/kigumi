package sem

import "kigumi/internal/syntax"

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
		c.errAtHelp(target, c.noCloneHelp(t, "take the element out with `.takeAt` instead"), cMoveIndex)
		return false
	}
	p, ok := c.buildPlace(target)
	if !ok {
		return true
	}
	if p.Deref {
		c.errAt(target, cMoveOutOfBorrow, t)
		return false
	}
	if !isField {
		return true
	}
	if root, ok := c.capturedFieldRoot(target); ok {
		c.errAt(target, cMoveCapturedField, c.placeName(target), c.r.Entities[root].Name)
		return false
	}
	if bn := c.r.Types.Node(baseType); bn.Kind == KNamed {
		if info := c.r.typeDecl(bn.Ent); info.Drop != 0 {
			name := c.placeName(target)
			c.errAt(target, cMoveDropType, name, baseType, name)
			return false
		}
	}
	return true
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
