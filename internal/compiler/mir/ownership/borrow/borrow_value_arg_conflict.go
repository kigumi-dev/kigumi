package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
)

func (c *borrowCheck) checkValueArgBorrows(in midir.Inst) {
	if in.Op != midir.OpCallValue || in.Ent != 0 || len(c.flowCallees(in)) > 0 {
		return
	}
	for i, a := range in.Args {
		b := c.holders[a]
		if b == nil || !b.hasOwner {
			continue
		}
		for _, o := range b.owners {
			if o.view || o.tied && !c.tieReaches(a, o.owner, c.curIdx) || !c.resetReaches(a, o.owner, c.curIdx) {
				continue
			}
			for j, other := range in.Args {
				if i == j {
					continue
				}
				for _, lent := range c.mutArgOwners(other) {
					if o.owner == lent.owner && ownquery.FieldPathsOverlap(o.path, lent.path) {
						c.reportTouch(in, a, b, o)
					}
				}
			}
		}
	}
}
