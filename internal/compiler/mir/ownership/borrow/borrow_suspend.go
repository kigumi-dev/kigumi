package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// checkSuspend rejects a borrow still live after a suspension point: the
// frame outlives the call that lent the value.
func (c *borrowCheck) checkSuspend(in midir.Inst, liveAfter map[midir.LocalID]bool) {
	if in.Op != midir.OpAwait {
		return
	}
	for holder := range liveAfter {
		b := c.holders[holder]
		if holder == in.Dst || b.empty || c.callerCells(b) {
			continue
		}
		l := holder
		if len(b.owners) > 0 {
			l = b.owners[0].owner
		}
		what := "a temporary"
		if c.f.Locals[l].Ent != 0 {
			what = "`" + c.f.Locals[l].Name + "`"
		}
		c.report(in.Node, holder, "borrow-across-await", "a borrow of "+what+" is still live across this `await`")
	}
}
