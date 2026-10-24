package sem

import (
	"kigumi/internal/diag"
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

func (c *checker) synthUnary(n syntax.NodeID, want TypeID) TypeID {
	u := c.t.View(n).AsUnary()
	op := u.Op().Kind()
	if isNumericLiteral(c.t, n) {
		return c.synthLiteral(n, want)
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
	} else if mut && !c.isMutablePlace(inner) {
		c.errAt(inner, cAssignNotPlace)
	}
	if mut {
		c.reserveMutBorrow(n, inner)
		c.invalidatePlace(inner)
		if root := c.rootOf(inner); root != 0 && c.capturedHere(root) {
			c.noteCaptureWrite(inner, root)
		}
	}
	c.crossBorrow(inner)
	refT := c.r.Types.Ref(t, mut)
	// A scalar `&mut` of a bare local/parameter can be stored (MIR
	// build_expr.go hir.Borrow): flag the local so it gets a cell to alias
	// into, instead of the borrow copying a disconnected snapshot.
	if mut {
		if _, ok := c.r.ScalarRefElem(refT); ok {
			ent, ok := c.identPlaceEnt(inner)
			switch {
			case ok && c.r.Types.Kind(c.r.Entities[ent].Type) == KRef:
				// `.value` through a reference reaches the referent's own cell; the local is not the storage.
			case ok:
				c.r.Entities[ent].Flags |= EfAddrTaken
				// Writes go through this borrow, not ent, so promote the
				// capture here instead of relying on assignStmt.
				c.noteCaptureWrite(inner, ent)
			case c.argAsyncPlace[n]:
				if !c.indexAssignUnsupported(inner) && c.isMutablePlace(inner) {
					c.errAt(inner, cMutPlaceAsyncArg)
				}
			case !c.argPlace[n] && !c.indexAssignUnsupported(inner) && c.isMutablePlace(inner):
				c.errAt(inner, cMutPlaceEscape)
			}
		}
	}
	return refT
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
