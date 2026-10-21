package sem

import "kigumi/internal/syntax"

// constPattern matches a value against a package-qualified constant, as `a.K` does in a literal pattern's place.
func (c *checker) constPattern(p syntax.NodeID, ent EntityID, scrutinee TypeID, subs []syntax.NodeID, seen map[string]syntax.NodeID) PatInfo {
	tt := c.r.Types
	e := &c.r.Entities[ent]
	for _, s := range subs {
		c.patternInner(s, TyPoison, false, seen)
	}
	if len(subs) > 0 {
		c.errAt(p, cPatternArity, e.Name, 0, plural(0), len(subs), plural(len(subs)))
		c.setType(p, TyPoison)
		return PatInfo{Kind: PatLiteral, Type: scrutinee}
	}
	got := c.vars.resolve(e.Type)
	_, hasValue := c.r.ConstValueOf(ent)
	switch {
	case scrutinee == TyPoison || got == TyPoison:
	case !hasValue || got != scrutinee && !(tt.Kind(got) == KUntyped && tt.IsNumeric(scrutinee)):
		c.errAt(p, cPatternLiteralType, e.Name, scrutinee)
		c.setType(p, TyPoison)
	}
	return PatInfo{Kind: PatLiteral, Type: scrutinee}
}
