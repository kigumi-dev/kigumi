package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
)

// placeCell reports a named local kept in a cell, shared with closures or lent as a scalar `&mut`: the cell is its
// storage, so it owns a borrow of its content like any local.
func (c *borrowCheck) placeCell(l midir.LocalID) bool {
	loc := c.f.Locals[l]
	return loc.Ent != 0 && loc.Cell && !loc.Borrowed
}

// paramCell reports a cell this body received from its caller: a closure's capture or a `mut self` receiver.
func (c *borrowCheck) paramCell(l midir.LocalID) bool {
	return c.f.Locals[l].Cell && slices.Contains(c.f.Params, l)
}

// callerCells reports a holder that only borrows cells a closure captured: the frame that owns them
// tracks what the closure returns, so only storing such a borrow is checked here.
func (c *borrowCheck) callerCells(b *borrowInfo) bool {
	return len(b.owners) > 0 && !slices.ContainsFunc(b.owners, func(o ownerRef) bool {
		return !c.placeCell(o.owner) || !slices.Contains(c.f.Params, o.owner)
	})
}
