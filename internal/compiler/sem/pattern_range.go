package sem

import (
	"math/big"

	"kigumi/internal/compiler/syntax"
)

// rangePattern types `lo..hi` / `lo..=hi` over an integer or Char scrutinee; both bounds are literals or named
// constants of its type. Ranges never make a match exhaustive.
func (c *checker) rangePattern(p syntax.NodeID, scrutinee TypeID) PatInfo {
	r := c.t.View(p).AsPatRange()
	lo, hi := r.Lo().ID(), r.Hi().ID()
	tt := c.r.Types
	if scrutinee != TyPoison && !tt.IsInteger(scrutinee) && scrutinee != TyChar {
		c.errAt(p, cRangePatternType, scrutinee)
		c.setType(p, TyPoison)
		scrutinee = TyPoison
	}
	okLo := c.rangeBoundValue(lo, scrutinee)
	okHi := c.rangeBoundValue(hi, scrutinee)
	if !okLo || !okHi {
		c.setType(p, TyPoison)
		return PatInfo{Kind: PatWild, Type: TyPoison}
	}
	if l, h := c.rangeBoundOrdinal(lo), c.rangeBoundOrdinal(hi); l != nil && h != nil && l.Cmp(h) > 0 {
		c.errAt(p, cRangePatternOrder)
	}
	return PatInfo{Kind: PatRange, Type: scrutinee}
}

// The grammar admits only an integer literal or bare identifier, so the one
// extra check beyond the literal-pattern rule is that an identifier resolves
// to a constant, not a binding it would otherwise shadow.
func (c *checker) rangeBoundValue(n syntax.NodeID, scrutinee TypeID) bool {
	got := c.vars.resolve(c.synth(n))
	if c.t.Kind(n) == syntax.Ident && got != TyPoison {
		ent := c.info.Uses[n]
		if ent == 0 || c.r.Entities[ent].Kind != EntConst {
			c.errAt(n, cRangeBoundNotConst)
			c.setType(n, TyPoison)
			return false
		}
		if c.r.ConstThunk(ent) != 0 {
			c.errAt(n, cComptimeConstUse, c.r.Entities[ent].Name)
			c.setType(n, TyPoison)
			return false
		}
	}
	tt := c.r.Types
	switch {
	case scrutinee == TyPoison || got == TyPoison:
		return false
	case tt.Kind(got) == KUntyped && c.adoptPendingPattern(n, scrutinee, got):
	case tt.Kind(got) == KUntyped:
		if !tt.IsInteger(scrutinee) && scrutinee != TyChar {
			return false
		}
		c.adopt(c.literalNode(n), scrutinee)
	case got != scrutinee:
		c.errAt(n, cPatternLiteralType, c.t.View(c.literalNode(n)).Tok().Text(), scrutinee)
		c.setType(n, TyPoison)
		return false
	}
	return c.info.Types[n] != TyPoison
}

// rangeBoundOrdinal is the value a checked bound stands for: an integer, or a character's code point.
func (c *checker) rangeBoundOrdinal(n syntax.NodeID) *big.Int {
	n = c.literalNode(n)
	if v, ok := c.info.Literals[n]; ok && v.Kind == constInt {
		return v.Int
	}
	switch c.t.Kind(n) {
	case syntax.CharLit:
		if ch, ok := syntax.DecodeChar(c.t.View(n).Tok().Text()); ok {
			return big.NewInt(int64(ch))
		}
	case syntax.Ident:
		if ent := c.info.Uses[n]; ent != 0 && c.r.Entities[ent].Kind == EntConst && c.r.constInfo(ent).Value.Kind == constChar {
			return big.NewInt(int64(c.r.constInfo(ent).Value.Char))
		}
	}
	return nil
}
