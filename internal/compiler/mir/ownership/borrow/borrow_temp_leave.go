package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/syntax"
)

// checkTempLeaves rejects a borrow of a temporary that leaves the `if` or `match` arm whose full expression made it:
// the temporary is released on its own path, while the join of the arms still holds the borrow.
func (c *borrowCheck) checkTempLeaves() {
	dropped, borrowAt := map[midir.LocalID]bool{}, map[midir.LocalID]syntax.NodeID{}
	for _, blk := range c.f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpDrop {
				dropped[in.Args[0]] = true
			}
			if b, ok := c.holders[in.Dst]; ok && in.Op == midir.OpBorrow {
				for _, o := range b.owners {
					if _, seen := borrowAt[o.owner]; o.temp && !seen {
						borrowAt[o.owner] = in.Node
					}
				}
			}
		}
	}
	var held []midir.LocalID
	for h := range c.holders {
		held = append(held, h)
	}
	slices.Sort(held)
	for _, h := range held {
		fh := c.f.Locals[h].FullExpr
		if c.f.Locals[h].Ent != 0 || fh == 0 || !c.multiDefined(h) {
			continue
		}
		for _, o := range c.holders[h].owners {
			if !o.temp || c.f.Locals[o.owner].FullExpr <= fh || dropped[o.owner] || c.scalarTemp(o.owner) || c.keptOwners[o.owner] {
				continue
			}
			n, ok := borrowAt[o.owner]
			if !ok {
				n = c.holders[h].node
			}
			c.noteKept(c.holders[h])
			c.reportCode(n, h, "temporary-borrow-leaves")
		}
	}
}

// noteKept records the temporaries b borrows once a kept borrow of them is reported.
func (c *borrowCheck) noteKept(b *borrowInfo) {
	if c.keptOwners == nil {
		c.keptOwners = map[midir.LocalID]bool{}
	}
	for _, o := range b.owners {
		if o.temp {
			c.keptOwners[o.owner] = true
		}
	}
}
