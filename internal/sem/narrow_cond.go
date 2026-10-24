package sem

import (
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

// condFacts extracts the true/false facts of a Bool condition.
func (c *checker) condFacts(n syntax.NodeID) (pos, neg []NarrowFact) {
	node := c.t.View(n)
	switch node.Kind() {
	case syntax.Paren:
		inner := node.AsParen().Inner().ID()
		pos, neg = c.condFacts(inner)
		c.setType(n, c.info.Types[inner])
		return pos, neg
	case syntax.Binary:
		bin := node.AsBinary()
		switch bin.Op().Kind() {
		case token.AndAnd:
			lp, _ := c.condFacts(bin.Left().ID())
			saved := c.snapshotFacts()
			c.addFacts(lp)
			c.branch++
			rp, _ := c.condFacts(bin.Right().ID())
			c.branch--
			c.facts = saved
			c.setType(n, TyBool)
			return append(lp, rp...), nil
		case token.EqEq, token.NotEq:
			c.condLeaf(n)
			return c.noneFacts(n)
		}
	case syntax.Unary:
		if operand := node.AsUnary().Operand().ID(); node.Tok().Kind() == token.Bang {
			c.negated = !c.negated
			pos, neg = c.condFacts(operand)
			c.negated = !c.negated
			if c.vars.resolve(c.info.Types[operand]) == TyBool {
				c.setType(n, TyBool)
				return neg, pos
			}
			c.synth(n)
			return nil, nil
		}
	case syntax.IsExpr:
		c.inCond++
		c.condLeaf(n)
		c.inCond--
		return c.isFacts(n)
	}
	c.condLeaf(n)
	return nil, nil
}

func (c *checker) condLeaf(n syntax.NodeID) TypeID {
	t := c.vars.resolve(c.synth(n))
	if c.r.Types.Kind(t) == KUntyped {
		t = c.adopt(c.literalNode(n), defaultOf(t))
	}
	if t == TyBool || t == TyPoison {
		return t
	}
	if c.isMethodValue(n) {
		c.mismatch(n, TyBool, t)
		return TyPoison
	}
	c.errAt(n, cConditionNotBool, t)
	return TyPoison
}

// isFacts also flags `p is Pattern` as always-true/false when existing
// facts already decide it.
func (c *checker) isFacts(n syntax.NodeID) (pos, neg []NarrowFact) {
	isExpr := c.t.View(n).AsIs()
	place := c.factPlace(isExpr.Left().ID())
	pos, neg = c.patternFacts(isExpr.Pattern().ID(), place)
	if len(pos) == 0 {
		return nil, nil
	}
	v := pos[0].Variant
	if is, isNot := c.knownVariant(place, v); is {
		c.errAt(n, cIsAlwaysTrue, c.placeName(isExpr.Left().ID()), c.r.Entities[v].Name)
	} else if isNot {
		c.errAt(n, cIsAlwaysFalse, c.placeName(isExpr.Left().ID()), c.r.Entities[v].Name)
	}
	return pos, neg
}

func (c *checker) noneFacts(n syntax.NodeID) (pos, neg []NarrowFact) {
	bin := c.t.View(n).AsBinary()
	lhs, rhs := bin.Left().ID(), bin.Right().ID()
	var other syntax.NodeID
	switch {
	case c.isBareNone(rhs):
		other = lhs
	case c.isBareNone(lhs):
		other = rhs
	default:
		return nil, nil
	}
	place := c.factPlace(other)
	none := c.noneVariant()
	if place == 0 || none == 0 {
		return nil, nil
	}
	is := []NarrowFact{{Place: place, Kind: FactIs, Variant: none}}
	isNot := []NarrowFact{{Place: place, Kind: FactIsNot, Variant: none}}
	if bin.Op().Kind() == token.EqEq {
		return is, isNot
	}
	return isNot, is
}
