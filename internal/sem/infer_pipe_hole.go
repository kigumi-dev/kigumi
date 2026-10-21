package sem

import "kigumi/internal/syntax"

// pipeArgs puts the left operand where the one `_` is written, else first; ok is false after reporting a second `_`.
func (c *checker) pipeArgs(call, lhs syntax.NodeID, args []syntax.NodeID) ([]syntax.NodeID, bool) {
	hole := -1
	for i, a := range args {
		if !c.isHole(a) {
			continue
		}
		if hole >= 0 {
			c.errAt(a, cPipePlaceholder, "this stage has more than one `_` placeholder")
			return nil, false
		}
		hole = i
	}
	if hole < 0 {
		for _, a := range args {
			if at := c.nestedHole(a); at != 0 {
				c.errAt(at, cPipePlaceholder, "`_` here is not a whole direct argument of this stage's call")
				return nil, false
			}
		}
		return append([]syntax.NodeID{lhs}, args...), true
	}
	out := append([]syntax.NodeID{}, args...)
	if c.t.Kind(args[hole]) == syntax.NamedArg {
		c.piped[args[hole]] = lhs
	} else {
		out[hole] = lhs
	}
	return out, true
}

func (c *checker) isHole(a syntax.NodeID) bool {
	if c.t.Kind(a) == syntax.NamedArg {
		a = c.t.View(a).AsNamedArg().Value().ID()
	}
	for c.t.Kind(a) == syntax.Paren {
		a = c.t.View(a).AsParen().Inner().ID()
	}
	return c.t.Kind(a) == syntax.Ident && c.t.View(a).Tok().Text() == "_"
}

// pipedFirst moves the left operand to the front of the evaluation order, wherever its placeholder put it.
func (p *CallPlan) pipedFirst(lhs syntax.NodeID) *CallPlan {
	at := -1
	for i, e := range p.Eval {
		if e == lhs {
			at = i
		}
	}
	if at <= 0 {
		return p
	}
	eval := append([]syntax.NodeID{lhs}, append(append([]syntax.NodeID{}, p.Eval[:at]...), p.Eval[at+1:]...)...)
	place := make([]int, len(p.Place))
	for i, k := range p.Place {
		switch {
		case k == at:
			place[i] = 0
		case k >= 0 && k < at:
			place[i] = k + 1
		default:
			place[i] = k
		}
	}
	return newCallPlan(eval, place)
}

// nestedHole finds a `_` value inside n that no `|>` stage inside n owns, or 0.
func (c *checker) nestedHole(n syntax.NodeID) syntax.NodeID {
	v := c.t.View(n)
	switch v.Kind() {
	case syntax.Ident:
		if v.Tok().Text() == "_" {
			return n
		}
	case syntax.Lambda:
		return 0
	case syntax.Binary:
		if b := v.AsBinary(); b.Op().Text() == "|>" {
			return c.nestedHole(b.Left().ID())
		}
	}
	for ch := range v.Children() {
		if at := c.nestedHole(ch.ID()); at != 0 {
			return at
		}
	}
	return 0
}
