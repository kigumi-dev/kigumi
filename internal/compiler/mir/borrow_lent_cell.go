package mir

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// asyncKept is, per argument of a call through a fn value, whether its Future or Task result may keep it (valueKept);
// nil when the result holds neither.
func (c *borrowCheck) asyncKept(in Inst) func(int) bool {
	if !c.holdsAsync(c.f.Locals[in.Dst].Type) {
		return nil
	}
	return c.valueKept(in)
}

// keptArgBorrow is what a result dst keeping argument a borrows: the place under a `&mut` write-back cell, also one an
// existential view boxes (keeping the view), else the argument's own borrows.
func (c *borrowCheck) keptArgBorrow(dst, a LocalID) (*borrowInfo, bool) {
	if d := c.definition(a); d != nil && d.Op == OpLendBox && len(d.Args) > 0 {
		if cd := c.definition(d.Args[0]); cd != nil && cd.Op == OpNewCell {
			b, ok := c.lentCellBorrow(d.Args[0], dst)
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
	return c.lentCellBorrow(a, dst)
}

// lentCellBorrow is the borrow under a `&mut` write-back cell, tied to dst from the write-back on; a scalar field or
// element's cell is read back at once, so a result keeping it would lose the write (E1122, as for a direct async call).
func (c *borrowCheck) lentCellBorrow(a, dst LocalID) (*borrowInfo, bool) {
	b, ok := c.cellBorrow(a)
	if !ok || c.scalarCell(a) {
		return nil, false
	}
	wb, ok := c.cellWriteBack(a)
	if !ok {
		return b, true
	}
	out := *b
	out.owners = slices.Clone(b.owners)
	for k := range out.owners {
		out.owners[k].tied = true
		c.addTieSite(dst, carrySite{block: c.resets.at.block, idx: wb, owner: out.owners[k].owner})
	}
	return &out, true
}

// cellWriteBack is where the call being checked writes a `&mut` write-back cell a, or the one a lends as a view, back.
func (c *borrowCheck) cellWriteBack(a LocalID) (int, bool) {
	if d := c.definition(a); d != nil && d.Op == OpLendBox && len(d.Args) > 0 {
		a = d.Args[0]
	}
	if d := c.definition(a); d == nil || d.Op != OpNewCell {
		return 0, false
	}
	at := c.resets.at
	insts := c.f.Blocks[at.block].Insts
	for i := at.idx + 1; i < len(insts); i++ {
		if insts[i].Op == OpCellTake && insts[i].Args[0] == a {
			return c.firstUse(insts[i].Dst, at.block, i)
		}
	}
	return 0, false
}

// cellPlaceBorrow is the place borrow under a `&mut` write-back cell a, with the view an existential lend box adds.
func (c *borrowCheck) cellPlaceBorrow(a LocalID) (*borrowInfo, bool) {
	cell := a
	if d := c.definition(a); d != nil && d.Op == OpLendBox && len(d.Args) > 0 {
		cell = d.Args[0]
	}
	b, ok := c.cellBorrow(cell)
	if !ok || cell == a {
		return b, ok
	}
	out := *b
	out.owners = append(slices.Clone(b.owners), ownerRef{owner: a, exact: true, view: true})
	return &out, true
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

// checkScalarTies reports E1122 at a scalar field or element `&mut` whose cell the call's result or a store would keep,
// spanning the borrowed place as sem does for a direct async call.
func (c *borrowCheck) checkScalarTies(in Inst) {
	t := c.p.R.Tree(c.f.File)
	for _, a := range in.Args {
		d := c.definition(a)
		if d == nil || !c.ties.scalarCells[a] {
			continue
		}
		n := d.Node
		if t.Kind(n) == syntax.BorrowExpr {
			n = t.View(n).AsBorrow().Operand().ID()
		}
		c.reportCatalog(n, a, "mut-place-async-arg")
	}
}
