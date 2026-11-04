package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// valueKept reports, per argument of value call in after the callee, whether its result may keep it: what the
// keptSummary of every origin the value can be says of the parameter the argument lands on, and yes for an unknown one.
func (c *borrowCheck) valueKept(in midir.Inst) func(int) bool {
	origins, ok := c.calleeOrigins(c.f, c.soleDefMap(), in.Args[0])
	if !ok || in.Ent != 0 || origins[0].isParam {
		return func(int) bool { return true }
	}
	return func(k int) bool {
		return slices.ContainsFunc(origins, func(o calleeOrigin) bool {
			return c.calleeKeeps(o.ent, o.argParam(k), o.params(in.Args[1:]))
		})
	}
}

// calleeKeeps reports whether fn item e called with args may keep its argument k in its result: an async body's Future
// keeps them all, and a body without a keptSummary is taken to.
func (c *borrowCheck) calleeKeeps(e sem.EntityID, k int, args []midir.LocalID) bool {
	if c.p.R.Entity(e).Kind != sem.EntFn || ownquery.IsAsyncBody(c.p.R, e) || k >= len(c.declaredParams(e)) {
		return true
	}
	s := c.keptBound(e, c.siteBind(e, args))
	return s == nil || s.keepsResult(k)
}

func (c *borrowCheck) defsOf(l midir.LocalID) []*midir.Inst {
	if c.defsAll == nil {
		c.defsAll = map[midir.LocalID][]*midir.Inst{}
		for b := range c.f.Blocks {
			for i := range c.f.Blocks[b].Insts {
				in := &c.f.Blocks[b].Insts[i]
				switch {
				case in.Op.Yields():
					c.defsAll[in.Dst] = append(c.defsAll[in.Dst], in)
				case in.Op == midir.OpCellSet:
					c.defsAll[in.Args[0]] = append(c.defsAll[in.Args[0]], in)
				}
			}
		}
	}
	return c.defsAll[l]
}
