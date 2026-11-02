package sem

import "kigumi/internal/compiler/syntax"

// rejectTempBorrowLet stops `let k = &mk()` (also through a field, `?.` field or index) when the temporary owns
// what its end-of-statement drop frees under k; every root but a binding that outlives the borrow is one.
func (c *checker) rejectTempBorrowLet(init syntax.NodeID) {
	if n, root, t, ok := c.tempBorrow(init); ok {
		c.errAt(n, cTempBorrowLet, c.temporaryName(root, t))
	}
}

// rejectTempBorrowStore is rejectTempBorrowLet for `place = &mk()`.
func (c *checker) rejectTempBorrowStore(rhs syntax.NodeID) {
	if n, root, t, ok := c.tempBorrow(rhs); ok {
		c.errAt(n, cTempBorrowStore, c.temporaryName(root, t))
	}
}

// tempBorrow finds the borrow init hands on as itself, through block tails and unsafe-style bodies; a join's
// arms are left to MIR, which follows each arm's temporary into every place that keeps it.
func (c *checker) tempBorrow(init syntax.NodeID) (syntax.NodeID, syntax.NodeID, TypeID, bool) {
	n := c.passThrough(init)
	if c.t.Kind(n) != syntax.BorrowExpr {
		return 0, 0, 0, false
	}
	operand := c.t.View(n).AsBorrow().Operand().ID()
	root, opt := operand, false
	for {
		root = c.t.Unparen(root)
		k := c.t.Kind(root)
		if k == syntax.MemberExpr && !c.namesEntity(root) {
			root = c.t.View(root).AsMember().Base().ID()
			continue
		}
		switch {
		case k == syntax.OptMemberExpr:
			root, opt = c.t.View(root).AsOptMember().Base().ID(), true
			continue
		case k == syntax.BracketExpr:
			root = c.t.View(root).AsBracket().Base().ID()
			continue
		case k == syntax.Ident || k == syntax.MemberExpr:
			if opt {
				root = c.t.Unparen(operand)
			} else if c.outlives(root, n) {
				return 0, 0, 0, false
			}
		case leafRules[k] == leafBody:
			if root = c.passThrough(root); !c.movedTail(root) {
				continue
			}
		}
		break
	}
	t := c.vars.resolve(c.info.Types[root])
	switch {
	case t == TyPoison || t == 0 || t == TyNever || c.r.Types.Kind(t) == KRef:
		return 0, 0, 0, false
	case c.r.Types.Kind(t) == KPrim && t != TyString && t != TyBytes:
		return 0, 0, 0, false
	}
	return n, root, t, true
}

// namesEntity reports whether member n names a declaration, as `pkg.c` or `T.V` do, rather than a field.
func (c *checker) namesEntity(n syntax.NodeID) bool {
	k := c.r.Entities[c.info.Uses[n]].Kind
	return k != EntNone && k != EntField
}

// outlives reports whether the root id names a place living past within: a static, or a local or context key
// declared or provided outside it. A const, variant, function or anything unresolved is a fresh value per use.
func (c *checker) outlives(id, within syntax.NodeID) bool {
	ent := c.info.Uses[id]
	switch c.r.Entities[ent].Kind {
	case EntStatic:
		return true
	case EntLocal, EntParam:
		return !c.within(c.r.Entities[ent].Node, within)
	case EntContextKey:
		b := c.info.CtxUse[id]
		return b.Local == 0 || !c.within(c.r.Entities[b.Local].Node, within)
	}
	return false
}

// movedTail reports whether n, a body's tail, names an owned local whose value moves into the body's result;
// moving a borrowed one is the error MIR reports.
func (c *checker) movedTail(n syntax.NodeID) bool {
	if c.t.Kind(n) != syntax.Ident || c.pathMode(n) != pathOwned || c.r.isCopy(c.vars.resolve(c.info.Types[n])) {
		return false
	}
	k := c.r.Entities[c.info.Uses[n]].Kind
	return k == EntLocal || k == EntParam
}

func (c *checker) temporaryName(root syntax.NodeID, t TypeID) string {
	if c.t.Kind(root) == syntax.CallExpr {
		callee := c.t.Unparen(c.t.View(root).AsCall().Callee().ID())
		if k := c.t.Kind(callee); k == syntax.Ident || k == syntax.MemberExpr {
			sp := c.t.Span(callee)
			return "the result of `" + string(c.t.File.Src[sp.Start:sp.End]) + "(..)`"
		}
	}
	return "a temporary `" + c.r.TypeString(t) + "`"
}
