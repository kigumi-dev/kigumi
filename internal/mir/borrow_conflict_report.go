package mir

import "slices"

// reportTouch reports in touching o.owner while holder still borrows it.
func (c *borrowCheck) reportTouch(in Inst, holder LocalID, b *borrowInfo, o ownerRef) {
	moves := in.Op == OpMove || in.Op == OpDrop || in.Op == OpFieldMove || c.takesPayload(in)
	passes := c.passesMutRef(in, o.owner)
	mutates := moves || (in.Op == OpBorrow && in.Index == 1) ||
		in.Op == OpSetField || in.Op == OpSetIndex || in.Op == OpCellSet || passes
	name := c.f.Locals[o.owner].Name
	switch {
	case c.tempScopeEnd(in, holder, b, o):
	case in.Op == OpDrop && in.EndScope:
		c.report(in.Node, o.owner, "move-while-borrowed", "`"+name+"` is still borrowed when its scope ends here")
	case moves:
		c.report(c.takeNode(in), o.owner, "move-while-borrowed", "`"+name+"` is borrowed and cannot be moved or dropped here")
	case o.mut:
		c.reportNoted(in.Node, o.owner, "borrow-conflict", "`"+name+"` is exclusively borrowed and cannot be used here", o.at(b), "`"+name+"` is borrowed here")
	case passes:
		c.reportHelped(in.Node, o.owner, "borrow-conflict", "`"+name+"` is borrowed and cannot be mutated here", o.at(b),
			"`"+name+"` is borrowed here, and the `&mut` argument may write through it",
			"read through `"+name+"` before the call (as in `"+name+".len()`) or borrow it after the call")
	case mutates || in.Dst == o.owner:
		c.reportHelped(in.Node, o.owner, "borrow-conflict", "`"+name+"` is borrowed and cannot be mutated here", o.at(b), "`"+name+"` is borrowed here", c.dropHeldHelp(holder))
	}
}

// dropHeldHelp names a holder nothing reads, whose release at the end of its scope, which may run its drop, keeps the borrow.
func (c *borrowCheck) dropHeldHelp(holder LocalID) string {
	h := c.f.Locals[holder]
	if h.Ent == 0 || h.Name == "" || !c.onlyDropped(holder) {
		return ""
	}
	return "`" + h.Name + "` keeps this borrow until its scope ends, since dropping it may run code; move `" + h.Name + "` into a block or pass it to a consuming call"
}

func (c *borrowCheck) onlyDropped(l LocalID) bool {
	for _, blk := range c.f.Blocks {
		if slices.Contains(blk.Term.Args, l) {
			return false
		}
		for _, in := range blk.Insts {
			if in.Op != OpDrop && slices.Contains(in.Args, l) {
				return false
			}
		}
	}
	return true
}
