package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (c *borrowCheck) futureLifetimeSource(in midir.Inst) (*borrowInfo, bool) {
	var slots []callSlot
	if e, args := c.flowCallee(in); e != 0 {
		slots = c.callSlots(c.p.R.Fn(e), args, 0)
	}
	var found []*borrowInfo
	for i, a := range in.Args[1:] {
		b, ok := c.holders[a]
		if !ok {
			continue
		}
		if !c.fitsDeclared(b, in.Dst) && !c.futureParamLifetime(slots, i, b) {
			return nil, false
		}
		if !slices.Contains(found, b) {
			found = append(found, b)
		}
	}
	switch len(found) {
	case 0:
		return nil, false
	case 1:
		return found[0], true
	}
	s := c.sink(in.Dst, in.Node)
	for _, b := range found {
		c.merge(s, b)
	}
	return s, true
}

// Future has no lifetime slot of its own, so an inherited borrow needs a named lifetime in the callee's signature.
func (c *borrowCheck) futureParamLifetime(slots []callSlot, i int, b *borrowInfo) bool {
	return b.lifetime != 0 && i < len(slots) && slices.ContainsFunc(slots[i].params, func(p sem.EntityID) bool {
		return c.p.R.TypeParam(p).IsLifetime
	})
}
