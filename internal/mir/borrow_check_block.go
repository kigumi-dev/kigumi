package mir

// checkBlock walks a block backward, tracking the holders live after each
// instruction, and reports the conflicts and escapes.
func (c *borrowCheck) checkBlock(bid BlockID) {
	blk := &c.f.Blocks[bid]
	live, real := copySet(c.liveOut[bid]), copySet(c.realOut[bid])
	for _, a := range blk.Term.Args {
		if _, ok := c.holders[a]; ok {
			real[a] = true
		}
		if b, ok := c.holders[a]; ok {
			if c.callerCells(b) || c.callerKept(b) {
				live[a] = true
				continue
			}
			// A holder with no owner is a borrow inherited from a
			// parameter, not one taken of a local: returning it is the
			// intended lifetime escape, and the caller's
			// own call site becomes the new holder (lifetimeSource).
			if !b.hasOwner {
				c.checkLifetimeEscape(a, b)
				live[a] = true
				continue
			}
			if lt := referentLifetime(b); lt != 0 {
				c.checkLifetimeEscape(a, &borrowInfo{lifetime: lt, node: b.node})
				live[a] = true
				continue
			}
			c.report(b.node, a, "borrow-escape", returnEscapeMessage(b))
			live[a] = true
		}
	}
	c.enterDefs(bid)
	c.enterBlock(bid)
	for i := len(blk.Insts) - 1; i >= 0; i-- {
		in := blk.Insts[i]
		c.curIdx = i
		c.checkInst(in, c.definedLive(live, bid, i), real)
		c.stepBack(in, live, c.plainRefRelease)
		c.stepBack(in, real, c.refRelease)
	}
}
