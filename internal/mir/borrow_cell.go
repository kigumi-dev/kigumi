package mir

import "slices"

// placelessBorrow is the holder of a borrow whose operand is no local's place: a statement's temporary, or a shared cell.
func (c *borrowCheck) placelessBorrow(in Inst) (*borrowInfo, bool) {
	if b, ok := c.tempBorrow(in); ok {
		return b, true
	}
	return c.cellBorrow(in)
}

// cellBorrow makes `&v` of a local shared with closures through a cell a holder of that cell,
// so storing it is checked like any borrow of a local.
func (c *borrowCheck) cellBorrow(in Inst) (*borrowInfo, bool) {
	d := c.definition(in.Args[0])
	if d == nil || d.Op != OpCellGet {
		return nil, false
	}
	if cell := c.f.Locals[d.Args[0]]; cell.Ent == 0 || cell.Borrowed {
		return nil, false
	}
	return &borrowInfo{owners: []ownerRef{{owner: d.Args[0], mut: in.Index == 1, exact: true, cell: true}}, hasOwner: true, node: in.Node}, true
}

// callerCells reports a holder that only borrows cells a closure captured: the frame that owns them
// tracks what the closure returns, so only storing such a borrow is checked here.
func (c *borrowCheck) callerCells(b *borrowInfo) bool {
	return len(b.owners) > 0 && !slices.ContainsFunc(b.owners, func(o ownerRef) bool {
		return !o.cell || !slices.Contains(c.f.Params, o.owner)
	})
}
