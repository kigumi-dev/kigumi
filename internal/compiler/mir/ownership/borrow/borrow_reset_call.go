package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// callReplaces reports whether in is a call that replaces what its `&mut` or `mut self` argument target holds on every path.
func (c *borrowCheck) callReplaces(in midir.Inst, target midir.LocalID) bool {
	if in.Op != midir.OpCall || in.Ent == 0 || c.p.R.Entity(in.Ent).Kind != sem.EntFn {
		return false
	}
	k := slices.Index(in.Args, target)
	return k >= 0 && c.analysis().storesParam(in.Ent, k)
}

// callResets reports whether in replaces on every path all of root, the named local fl's target borrows.
func (c *borrowCheck) callResets(in midir.Inst, fl storeFlow, root midir.LocalID) bool {
	l := fl.target
	for c.f.Locals[l].Ent == 0 {
		d := c.definition(l)
		switch {
		case d == nil || len(d.Args) == 0:
			return false
		case d.Op == midir.OpBorrow:
			r, path, exact, ok := c.projectionPlace(d.Args[0])
			return ok && r == root && exact && len(path) == 0 && len(c.storeDests(fl)) == 1 && c.callReplaces(in, fl.target)
		case d.Op != midir.OpNewCell && d.Op != midir.OpShare && d.Op != midir.OpCopy && d.Op != midir.OpMove && d.Op != midir.OpAlias:
			return false
		}
		l = d.Args[0]
	}
	return false
}

// noteCallReset makes a replacing call a reset of the root it stores into, at its write-back when there is one.
func (c *borrowCheck) noteCallReset(in midir.Inst, fl storeFlow, root midir.LocalID, b *borrowInfo) {
	if c.f.Locals[root].Ent == 0 || !c.callResets(in, fl, root) {
		return
	}
	at := c.resets.at
	if idx, ok := c.tieSite(fl.target, root); ok {
		at.idx = idx
	}
	if c.resets.ends == nil {
		c.resets.ends = map[carrySite]bool{}
	}
	c.resets.ends[carrySite{block: at.block, idx: at.idx, owner: root}] = true
	saved := c.resets.at
	c.resets.at = at
	c.noteResetSite(root, b)
	c.resets.at = saved
}

// resetsAt reports whether insts[i] of block bid ends every borrow holder had: a replacing field store or call.
func (c *borrowCheck) resetsAt(bid midir.BlockID, i int, holder midir.LocalID) bool {
	return c.replacesHolder(c.f.Blocks[bid].Insts[i], holder) || c.resets.ends[carrySite{block: bid, idx: i, owner: holder}]
}
