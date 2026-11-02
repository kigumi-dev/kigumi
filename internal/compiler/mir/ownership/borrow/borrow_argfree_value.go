package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// valueKept reports, per argument of value call in after the callee, whether its result may keep it: what the
// keptSummary of every fn item the value can be says (calleeKeeps), and yes for any other callee.
func (c *borrowCheck) valueKept(in midir.Inst) func(int) bool {
	ents := c.fnItemOrigins(in.Args[0], 8)
	if !ents.known || in.Ent != 0 {
		return func(int) bool { return true }
	}
	return func(k int) bool {
		return slices.ContainsFunc(ents.ents, func(e sem.EntityID) bool { return c.calleeKeeps(e, k, in.Args[1:]) })
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
