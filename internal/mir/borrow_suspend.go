package mir

// checkSuspend rejects a borrow still live after a suspension point: the
// frame outlives the call that lent the value.
func (c *borrowCheck) checkSuspend(in Inst, liveAfter map[LocalID]bool) {
	if in.Op != OpAwait {
		return
	}
	for holder := range liveAfter {
		b := c.holders[holder]
		if holder == in.Dst || b.empty {
			continue
		}
		name := c.f.Locals[holder].Name
		if len(b.owners) > 0 {
			name = c.f.Locals[b.owners[0].owner].Name
		}
		c.report(in.Node, holder, "borrow-across-await", "a borrow of `"+name+"` is still live across this `await`")
	}
}
