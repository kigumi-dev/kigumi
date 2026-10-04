package sem

import (
	"kigumi/internal/diag"
	"kigumi/internal/syntax"
)

func (c *checker) errAt(n syntax.NodeID, code Code, args ...any) bool {
	return c.r.errAt(c.f, n, code, args...)
}

func (c *checker) errFix(n syntax.NodeID, fix diag.Fix, code Code, args ...any) bool {
	return c.r.errFix(c.f, n, fix, code, args...)
}

func (c *checker) loc(n syntax.NodeID) diag.Location { return diag.At(c.t.File, c.t.Span(n)) }

func (c *checker) replaceNode(n syntax.NodeID, title, text string) diag.Fix {
	return diag.Fix{Title: title, Loc: c.loc(n), NewText: text}
}
