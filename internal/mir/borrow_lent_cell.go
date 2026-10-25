package mir

import "slices"

// asyncValueCall reports whether a call through a fn value may build an async body's Future, which keeps its arguments:
// its result holds a Future or Task and the value is not known to be a synchronous function item (syncFnValue).
func (c *borrowCheck) asyncValueCall(in Inst) bool {
	if !c.holdsAsync(c.f.Locals[in.Dst].Type) {
		return false
	}
	return !c.syncFnValue(in.Args[0], 8)
}

// lentArgBorrow is what a value call's argument lends its result: for an async value call, the place under a `&mut`
// write-back cell, also one an existential view boxes (keeping the view); else the argument's own borrows.
func (c *borrowCheck) lentArgBorrow(in Inst, a LocalID) (*borrowInfo, bool) {
	if !c.asyncValueCall(in) {
		b, ok := c.holders[a]
		return b, ok
	}
	if d := c.definition(a); d != nil && d.Op == OpLendBox && len(d.Args) > 0 {
		if cd := c.definition(d.Args[0]); cd != nil && cd.Op == OpNewCell {
			b, ok := c.lentCellBorrow(d.Args[0], in.Dst)
			if !ok {
				return nil, false
			}
			out := *b
			out.owners = append(slices.Clone(b.owners), ownerRef{owner: a, exact: true, view: true})
			return &out, true
		}
	}
	if b, ok := c.holders[a]; ok {
		return b, true
	}
	return c.lentCellBorrow(a, in.Dst)
}

// lentCellBorrow is the borrow under a `&mut` write-back cell, tied to dst from the write-back on; a scalar field or
// element's cell is read back at once, so a result keeping it would lose the write (E1122, as for a direct async call).
func (c *borrowCheck) lentCellBorrow(a, dst LocalID) (*borrowInfo, bool) {
	b, ok := c.cellBorrow(a)
	if !ok || c.scalarCell(a) {
		return nil, false
	}
	at := c.resets.at
	insts := c.f.Blocks[at.block].Insts
	for i := at.idx + 1; i < len(insts); i++ {
		if insts[i].Op != OpCellTake || insts[i].Args[0] != a {
			continue
		}
		if wb, ok := c.firstUse(insts[i].Dst, at.block, i); ok {
			out := *b
			out.owners = slices.Clone(b.owners)
			for k := range out.owners {
				out.owners[k].tied = true
				c.addTieSite(dst, carrySite{block: at.block, idx: wb, owner: out.owners[k].owner})
			}
			return &out, true
		}
	}
	return b, true
}

// scalarCell notes a as a scalar field or element `&mut` cell that something outliving the call takes in, for E1122.
func (c *borrowCheck) scalarCell(a LocalID) bool {
	if _, ok := c.cellBorrow(a); !ok {
		return false
	}
	if _, scalar := c.p.R.ScalarRefElem(c.f.Locals[a].Type); !scalar {
		return false
	}
	if c.ties.scalarCells == nil {
		c.ties.scalarCells = map[LocalID]bool{}
	}
	c.ties.scalarCells[a] = true
	return true
}

func (c *borrowCheck) cellBorrow(a LocalID) (*borrowInfo, bool) {
	if d := c.definition(a); d == nil || d.Op != OpNewCell {
		return nil, false
	}
	for d := c.definition(a); d != nil && len(d.Args) > 0; d = c.definition(d.Args[0]) {
		switch d.Op {
		case OpBorrow:
			b, ok := c.holders[d.Dst]
			return b, ok
		case OpNewCell, OpShare, OpCopy, OpAlias:
		default:
			return nil, false
		}
	}
	return nil, false
}

// firstUse is the index of the first instruction after i in block bid that reads l.
func (c *borrowCheck) firstUse(l LocalID, bid BlockID, i int) (int, bool) {
	insts := c.f.Blocks[bid].Insts
	for j := i + 1; j < len(insts); j++ {
		if slices.Contains(insts[j].Args, l) {
			return j, true
		}
	}
	return 0, false
}

// checkScalarTies reports E1122 at a scalar field or element `&mut` whose cell the call's result or a store would keep.
func (c *borrowCheck) checkScalarTies(in Inst) {
	for _, a := range in.Args {
		if d := c.definition(a); c.ties.scalarCells[a] && d != nil {
			c.reportCode(d.Node, a, "mut-place-async-arg")
		}
	}
}
