package sem

import (
	"strings"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// takeName names a taken place on one line: as written when that fits, else with each base that
// spans lines shown as `(...)`, as is a base that is no place or call (a join, a block).
func (c *checker) takeName(n syntax.NodeID) string {
	n = c.t.Unparen(n)
	v := c.t.View(n)
	switch c.t.Kind(n) {
	case syntax.Ident, syntax.MemberExpr, syntax.OptMemberExpr, syntax.CallExpr, syntax.WsCallExpr, syntax.BracketExpr:
	default:
		return "(...)"
	}
	if s := c.fullText(n); !strings.Contains(s, "\n") {
		return s
	}
	switch c.t.Kind(n) {
	case syntax.MemberExpr:
		return c.takeName(v.AsMember().Base().ID()) + "." + v.Tok().Text()
	case syntax.OptMemberExpr:
		return c.takeName(v.AsOptMember().Base().ID()) + "?." + v.Tok().Text()
	case syntax.BracketExpr:
		return c.takeName(v.AsBracket().Base().ID()) + "[..]"
	}
	return "(...)"
}

// fullText is n's source with the closer that a bracket's or call's span leaves out.
func (c *checker) fullText(n syntax.NodeID) string {
	sp := c.t.Span(n)
	open := uint32(0)
	switch c.t.Kind(n) {
	case syntax.BracketExpr:
		open = c.t.View(n).AsBracket().Open().Index()
	case syntax.CallExpr:
		if h := c.t.View(n).AsCall().Head(); h.Kind() == token.LParen {
			open = h.Index()
		}
	}
	if end, ok := c.closerEnd(open); ok && end > sp.End {
		sp.End = end
	}
	return string(c.t.File.Src[sp.Start:sp.End])
}

// closerEnd is where the bracket matching the opener at token index open ends.
func (c *checker) closerEnd(open uint32) (token.Pos, bool) {
	if open == 0 {
		return 0, false
	}
	depth := 0
	for i := int(open); i < len(c.t.Toks); i++ {
		switch c.t.Toks[i].Kind {
		case token.LParen, token.LBracket, token.LBrace:
			depth++
		case token.RParen, token.RBracket, token.RBrace:
			if depth--; depth == 0 {
				return c.t.Toks[i].End, true
			}
		case token.EOF:
			return 0, false
		}
	}
	return 0, false
}
