package mir

import (
	"slices"

	"kigumi/internal/syntax"
)

// reportTouch reports in touching o.owner while holder still borrows it.
func (c *borrowCheck) reportTouch(in Inst, holder LocalID, b *borrowInfo, o ownerRef) {
	// A `&mut` argument's write-back reports at the borrow that lent it, so the argument gets one diagnostic.
	if site, ok := c.writeBackSite(in); ok {
		in.Node = site
	}
	moves := in.Op == OpMove || in.Op == OpDrop || in.Op == OpFieldMove || in.Op == OpCellTake || c.takesPayload(in)
	passes := c.passesMutRef(in, o.owner)
	_, runs := c.cellRunHit(in, o)
	mutates := moves || (in.Op == OpBorrow && in.Index == 1) ||
		in.Op == OpSetField || in.Op == OpSetIndex || in.Op == OpCellSet || passes || runs
	name := c.f.Locals[o.owner].Name
	switch {
	case runs && c.replacesDropping(in) && !c.touchesDirect(in, o.owner):
		c.reportHelped(in.Node, o.owner, "borrow-conflict", "`"+name+"` is borrowed and cannot be mutated here", o.at(b), "`"+name+"` is borrowed here",
			"this store drops the value it replaces, whose `drop` may call a closure writing `"+name+"`; end the borrow before the store")
	case runs && in.Op == OpDrop && in.Args[0] != o.owner:
		dropped := c.f.Locals[in.Args[0]].Name
		if dropped == "" {
			dropped = "a value"
		} else {
			dropped = "`" + dropped + "`"
		}
		c.reportHelped(in.Node, o.owner, "borrow-conflict", "`"+name+"` is borrowed and cannot be mutated here", o.at(b), "`"+name+"` is borrowed here",
			"dropping "+dropped+" here runs a `drop` that may call a closure writing `"+name+"`; end the borrow before it is dropped")
	case c.tempScopeEnd(in, holder, b, o):
	case in.Op == OpDrop && in.EndScope:
		c.report(in.Node, o.owner, "move-while-borrowed", "`"+name+"` is still borrowed when its scope ends here")
	case moves:
		c.report(c.takeNode(in), o.owner, "move-while-borrowed", "`"+name+"` is borrowed and cannot be moved or dropped here")
	case o.mut:
		c.reportNoted(in.Node, o.owner, "borrow-conflict", "`"+name+"` is exclusively borrowed and cannot be used here", o.at(b), "`"+name+"` is borrowed here")
	case runs && taskClose(in):
		c.reportHelped(in.Node, o.owner, "borrow-conflict", "`"+name+"` is still borrowed where this body ends, and a task it spawned, which runs there, writes it", o.at(b), "`"+name+"` is borrowed here",
			"await the task before taking the borrow, or end the borrow before the body ends (a `defer` keeps it live)")
	case runs:
		c.reportHelped(in.Node, o.owner, "borrow-conflict", "`"+name+"` is borrowed and cannot be mutated here", o.at(b), "`"+name+"` is borrowed here",
			"this call runs a closure that writes `"+name+"`; end the borrow before the call")
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

func (c *borrowCheck) touchesDirect(in Inst, owner LocalID) bool {
	touched, _, _ := c.touch(in, owner)
	return touched
}

// writeBackSite is the node of the borrow whose `&mut` cell in stores back into its place, if in is such a store.
func (c *borrowCheck) writeBackSite(in Inst) (syntax.NodeID, bool) {
	switch in.Op {
	case OpCopy, OpCellSet, OpSetField, OpSetIndex, OpGlobalSet:
	default:
		return 0, false
	}
	v := in.Args[len(in.Args)-1]
	d := c.definition(v)
	for d != nil && d.Op == OpAlias {
		d = c.definition(d.Args[0])
	}
	if d == nil || d.Op != OpCellTake && d.Op != OpCellGet {
		return 0, false
	}
	cell := c.definition(d.Args[0])
	if cell == nil || cell.Op != OpNewCell {
		return 0, false
	}
	return cell.Node, true
}
