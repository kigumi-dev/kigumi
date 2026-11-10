package sem

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

func (c *checker) synthUnary(n syntax.NodeID, want TypeID) TypeID {
	u := c.t.View(n).AsUnary()
	op := u.Op().Kind()
	if isNumericLiteral(c.t, n) {
		return c.synthLiteral(n, want)
	}
	if t, ok := c.foldUntypedUnary(n, want); ok {
		return t
	}
	operand := c.unaryOperand(u.Operand().ID(), op, want)
	tt := c.r.Types
	operand = c.readThrough(c.vars.resolve(operand))
	if operand == TyPoison {
		return TyPoison
	}
	if tt.Kind(operand) == KUntyped {
		operand = c.adopt(c.literalNode(u.Operand().ID()), c.unaryLiteralTarget(operand, op, want))
		if operand == TyPoison {
			return TyPoison
		}
	}
	switch op {
	case token.Minus:
		if tt.IsSigned(operand) {
			if tt.IsInteger(operand) {
				// Negating the type's minimum value overflows (checked_i64 traps).
				c.edge(EffectEdge{Kind: EdgePanic, Node: n})
			}
			return operand
		}
		if tt.IsInteger(operand) {
			c.errAt(n, cUnaryMinusUnsigned, operand)
			return TyPoison
		}
	case token.Bang:
		if operand == TyBool {
			return TyBool
		}
	case token.Tilde:
		if tt.IsInteger(operand) {
			return operand
		}
	}
	return c.userOperator(n, u.Op().Text(), []TypeID{operand}, []syntax.NodeID{u.Operand().ID()})
}

func (c *checker) synthBorrow(n syntax.NodeID, want TypeID) TypeID {
	b := c.t.View(n).AsBorrow()
	inner := b.Operand().ID()
	t := c.synthPlace(inner)
	c.rejectJoinBase(inner)
	if w := c.r.Types.Node(c.vars.resolve(want)); w.Kind == KRef && c.r.Types.Kind(c.vars.resolve(t)) == KUntyped {
		target := c.vars.resolve(w.Elem)
		if c.r.Types.ContainsVar(target) || c.r.Types.Kind(target) == KUntyped {
			target = defaultOf(c.vars.resolve(t))
		}
		t = c.adopt(c.literalNode(inner), target)
	}
	mut := b.Flags()&syntax.FlagMut != 0
	if staticEnt, ok := c.staticRootOf(inner); ok && c.atomicPlace[n] {
	} else if ok && c.r.Static(staticEnt).Mut {
		name := c.r.Entities[staticEnt].Name
		c.errAt(n, cStaticMutOrdinaryRef, name, name)
	} else if ok && mut {
		c.errAt(n, cStaticBorrowMut, c.r.Entities[staticEnt].Name)
	} else if mut && c.indexAssignUnsupported(inner) {
		c.errAt(inner, cIndexAssignUnsupported)
	} else if mut && c.rejectCtxKeyWrite(inner, "borrowed with `&mut`") {
	} else if mut && c.rejectViewAccess(inner, accessMutBorrow) {
	} else if mut && !c.isMutablePlace(inner) {
		c.errAt(inner, cAssignNotPlace)
	}
	if mut {
		c.reserveMutBorrow(n, inner)
		c.invalidatePlace(inner)
		if root := c.rootOf(inner); root != 0 && c.capturedHere(root) {
			c.noteCaptureWrite(inner, root, false)
		} else if root != 0 {
			c.shareOutsideWrite(inner, root)
		}
	}
	c.crossBorrow(inner)
	refT := c.r.Types.Ref(t, mut)
	if mut {
		c.noteScalarBorrow(n, inner, refT)
	}
	return refT
}

// defaultBorrowedLiteral gives `let r = &5` the literal's default type, as `let r: &Int = &5` gets its declared one.
func (c *checker) defaultBorrowedLiteral(b syntax.NodeID, got TypeID) TypeID {
	r := c.r.Types.Node(c.vars.resolve(got))
	if r.Kind != KRef || c.r.Types.Kind(c.vars.resolve(r.Elem)) != KUntyped {
		return got
	}
	elem := c.adopt(c.literalNode(c.t.View(b).AsBorrow().Operand().ID()), defaultOf(c.vars.resolve(r.Elem)))
	return c.setType(b, c.r.Types.Ref(elem, r.Flags&flagMut != 0))
}

func (c *checker) noteDiag(d diag.Diagnostic) { c.r.emitDiag(c.f, d) }

func (c *checker) unaryOperand(n syntax.NodeID, op token.Kind, want TypeID) TypeID {
	if ctx := c.shiftContext(want); op == token.Minus && ctx != 0 && c.info.Types[n] == 0 && c.shiftShaped(n) {
		return c.setType(n, c.synthKind(n, ctx))
	}
	return c.synthRead(n)
}

func (c *checker) unaryLiteralTarget(operand TypeID, op token.Kind, want TypeID) TypeID {
	tt := c.r.Types
	w := c.vars.resolve(want)
	switch {
	case op == token.Tilde && operand != TyUntypedInt:
	case operand == TyUntypedInt && tt.IsInteger(w), op == token.Minus && tt.IsFloat(w):
		return w
	}
	return defaultOf(operand)
}
