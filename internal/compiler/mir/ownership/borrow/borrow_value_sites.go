package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
)

// valueCallSites maps, for argument k of value call in, the sites of every body it may run as a direct call to that
// body maps them; false unless each callee is a body here.
func (c *borrowCheck) valueCallSites(g *midir.Func, defs map[midir.LocalID]*midir.Inst, in midir.Inst, k int, kind siteKind) ([]paramStore, bool) {
	if in.Op != midir.OpCallValue || k == 0 {
		return nil, false
	}
	origins, ok := c.calleeOrigins(g, defs, in.Args[0])
	if !ok || slices.ContainsFunc(origins, func(o calleeOrigin) bool { return o.body == nil }) {
		return nil, false
	}
	var out []paramStore
	for _, o := range origins {
		params := localsOf(o.params(in.Args[1:]))
		for _, s := range c.paramSites(paramKey{g: o.body, i: o.argParam(k - 1), kind: kind}) {
			out = append(out, c.relaySite(g, defs, s, params, kind, 8)...)
		}
	}
	return out, true
}
