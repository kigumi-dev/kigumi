package sem

import "kigumi/internal/compiler/syntax"

type patPart struct {
	pat, expr syntax.NodeID
}

// literalParts pairs each sub-pattern with the part of the literal it destructures, when p and expr have the same shape.
func (c *checker) literalParts(p, expr syntax.NodeID) ([]patPart, bool) {
	switch {
	case c.t.Kind(p) == syntax.PatTuple && c.t.Kind(expr) == syntax.TupleLit:
		return zipParts(c.t.List(p), c.t.List(expr)), true
	case c.t.Kind(p) == syntax.PatRecord && c.t.Kind(expr) == syntax.RecordLit:
		return c.recordLitParts(p, expr)
	case c.t.Kind(p) == syntax.PatCtor && c.t.Kind(expr) == syntax.CallExpr:
		call, ok := c.info.Calls[expr]
		args := c.t.View(expr).AsCall().Args()
		if !ok || call.Kind != CallVariant || call.Callee != c.info.Uses[p] {
			return nil, false
		}
		for a := range args.IDs() {
			if c.t.Kind(a) == syntax.NamedArg || c.t.Kind(a) == syntax.Spread {
				return nil, false
			}
		}
		return zipParts(c.t.View(p).AsPatCtor().Args(), args), true
	}
	return nil, false
}

func zipParts(pats, items syntax.ListView) []patPart {
	var out []patPart
	for i := 0; i < pats.Len() && i < items.Len(); i++ {
		out = append(out, patPart{pats.ItemID(i), items.ItemID(i)})
	}
	return out
}

// A shorthand field pattern binds on the field node itself.
func (c *checker) recordLitParts(p, expr syntax.NodeID) ([]patPart, bool) {
	entries := map[string]syntax.NodeID{}
	for e := range c.t.View(expr).AsRecordLit().Fields().IDs() {
		en := c.t.View(e)
		if en.Kind() == syntax.Spread {
			return nil, false
		}
		entries[en.Tok().Text()] = en.ArgValue().ID()
	}
	var out []patPart
	for f := range c.t.View(p).AsPatRecord().Fields().IDs() {
		fv := c.t.View(f).AsPatField()
		sub := f
		if fv.Sub().Valid() {
			sub = fv.Sub().ID()
		}
		if v, ok := entries[fv.Name().Text()]; ok {
			out = append(out, patPart{sub, v})
		}
	}
	return out, true
}
