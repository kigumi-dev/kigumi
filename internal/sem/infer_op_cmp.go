package sem

import (
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

func (c *checker) comparisonMismatch(n, rhs syntax.NodeID, l, r TypeID) TypeID {
	if c.r.Types.IsNumeric(l) && c.r.Types.IsNumeric(r) {
		c.errAtHelp(n, mixedUsizeHelp(l, r), cNumericMixed, l, r)
	} else {
		c.mismatch(rhs, l, r)
	}
	return TyPoison
}

func (c *checker) isBareNone(n syntax.NodeID) bool {
	n = c.literalNode(n)
	if c.t.Kind(n) != syntax.Ident {
		return false
	}
	ent := c.info.Uses[n]
	return ent != 0 && c.r.Entities[ent].Kind == EntVariant && c.r.Entities[ent].Name == "None" && c.r.Entities[ent].Parent == c.r.Types.optionEnt
}

// Two literals share one pending variable so an index position can still make them usize.
func (c *checker) synthRange(n syntax.NodeID, op token.Kind, lhs, rhs syntax.NodeID, l, r TypeID) TypeID {
	tt := c.r.Types
	lu, ru := tt.Kind(l) == KUntyped, tt.Kind(r) == KUntyped
	switch {
	case lu && ru:
		if l == TyUntypedFloat || r == TyUntypedFloat {
			c.errAt(n, cRangeOperands)
			return TyPoison
		}
		v := c.vars.fresh(n, "T")
		c.vars.unify(TyUntypedInt, v)
		i, _ := c.vars.index(v)
		c.vars.notePending(i, c.literalNode(lhs))
		c.vars.notePending(i, c.literalNode(rhs))
		l, r = v, v
	case lu && l == TyUntypedInt && c.pendingIntVar(r):
		i, _ := c.vars.index(r)
		c.vars.notePending(i, c.literalNode(lhs))
		l = r
	case ru && r == TyUntypedInt && c.pendingIntVar(l):
		i, _ := c.vars.index(l)
		c.vars.notePending(i, c.literalNode(rhs))
		r = l
	case lu && tt.IsInteger(r):
		l = c.adopt(c.literalNode(lhs), r)
	case ru && tt.IsInteger(l):
		r = c.adopt(c.literalNode(rhs), l)
	}
	if l == TyPoison || r == TyPoison {
		return TyPoison
	}
	if _, isVar := c.vars.index(l); l != r || !isVar && !tt.IsInteger(l) {
		c.errAt(n, cRangeOperands)
		return TyPoison
	}
	rng := c.r.langItem(c.f, n, "Range")
	if rng == 0 {
		return TyPoison
	}
	c.info.Calls[n] = CallInfo{Kind: CallBuiltinOp, Op: op, OpText: op.String()}
	c.touched = append(c.touched, n)
	return tt.Named(rng, []TypeID{l})
}

// Treats a borrow of a Copy value as the value itself, so `&Int` works in arithmetic, comparison and interpolation.
func (c *checker) readThrough(t TypeID) TypeID {
	if n := c.r.Types.Node(t); n.Kind == KRef && c.r.isCopy(n.Elem) {
		return n.Elem
	}
	return t
}

func (c *checker) pendingIntVar(t TypeID) bool {
	i, ok := c.vars.pendingIndex(t)
	return ok && c.vars.pending[i] == 1
}
