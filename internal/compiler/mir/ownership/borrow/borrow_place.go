package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
)

// placeBorrow holds the place in.Args[0] reads under root. A borrow of a fresh slice itself also holds
// the slice, which ends with its statement; an element under it is the root's own object.
func (c *borrowCheck) placeBorrow(in midir.Inst, root midir.LocalID, path ownquery.Path, exact bool) *borrowInfo {
	mut := in.Index == 1
	ref := c.endedOwner(ownerRef{owner: root, mut: mut, path: path, exact: exact, lifetime: c.projectionLifetime(root, in.Args[0])}, in.Args[0])
	b := &borrowInfo{owners: []ownerRef{ref}, hasOwner: true, node: in.Node}
	if slice, ok := c.borrowedSlice(in.Args[0]); ok {
		b.owners = append(b.owners, ownerRef{owner: slice, mut: mut, exact: true, temp: true})
	}
	mergeInto(b, c.throughRef(root, path, exact, mut))
	return b
}

// borrowedSlice: l is a fresh slice, through copies only.
func (c *borrowCheck) borrowedSlice(l midir.LocalID) (midir.LocalID, bool) {
	for c.f.Locals[l].Ent == 0 {
		d := c.definition(l)
		switch {
		case d == nil:
			return 0, false
		case c.freshSlice(d):
			return l, true
		case d.Op != midir.OpCopy:
			return 0, false
		}
		l = d.Args[0]
	}
	return 0, false
}
