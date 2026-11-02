package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// endedOwner is the owner a borrow of arg, a place under ref's root, depends on: the root while it lives, else the
// copy arg read out of it before its end, which lasts only as long as the full expression that made it.
func (c *borrowCheck) endedOwner(ref ownerRef, arg midir.LocalID) ownerRef {
	if !c.endedAt(ref.owner) {
		return ref
	}
	if c.f.Locals[arg].Ent == 0 {
		ref = ownerRef{owner: arg, mut: ref.mut, exact: true}
	}
	ref.temp = true
	return ref
}

// endedAt reports whether root may already be gone at the instruction being collected: on some path to it a
// drop or whole move of root follows its last definition, so a borrow there holds only a statement's copy.
func (c *borrowCheck) endedAt(root midir.LocalID) bool {
	if c.ended == nil {
		c.ended = c.state.endedOf(c.f)
	}
	return c.ended.before(root, c.resets.at.block, c.resets.at.idx)
}
