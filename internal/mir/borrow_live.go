package mir

func uses(in Inst) []LocalID { return in.Args }

// liveness computes, per block, the borrow holders live at exit.
func (c *borrowCheck) liveness() {
	c.liveOut, c.liveIn = c.solveLive(c.plainRefRelease)
	c.realOut, _ = c.solveLive(c.refRelease)
}

func (c *borrowCheck) solveLive(skip func(LocalID) bool) (liveOut, liveIn []map[LocalID]bool) {
	n := len(c.f.Blocks)
	liveOut = make([]map[LocalID]bool, n)
	liveIn = make([]map[LocalID]bool, n)
	for i := range n {
		liveOut[i] = map[LocalID]bool{}
		liveIn[i] = map[LocalID]bool{}
	}
	for changed := true; changed; {
		changed = false
		for bid := n - 1; bid >= 0; bid-- {
			blk := &c.f.Blocks[bid]
			out := map[LocalID]bool{}
			for _, t := range blk.Term.Targets {
				in := liveIn[t]
				if k, ok := c.threadedTarget(BlockID(bid), t); ok {
					in = c.liveThrough(t, liveIn[k], skip)
				}
				for l := range in {
					out[l] = true
				}
			}
			live := c.liveThrough(BlockID(bid), out, skip)
			if !sameSet(out, liveOut[bid]) || !sameSet(live, liveIn[bid]) {
				changed = true
			}
			liveOut[bid] = out
			liveIn[bid] = live
		}
	}
	return liveOut, liveIn
}

func copySet(a map[LocalID]bool) map[LocalID]bool {
	out := make(map[LocalID]bool, len(a))
	for k := range a {
		out[k] = true
	}
	return out
}

func sameSet(a, b map[LocalID]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// definitions computes, per block, the live-in holders assigned on some path into it.
func (c *borrowCheck) definitions() {
	n := len(c.f.Blocks)
	c.definedIn = make([]map[LocalID]bool, n)
	for i := range n {
		c.definedIn[i] = map[LocalID]bool{}
	}
	for changed := true; changed; {
		changed = false
		for bid := range n {
			blk := &c.f.Blocks[bid]
			var defs []LocalID
			for _, in := range blk.Insts {
				if _, ok := c.holders[in.Dst]; ok && shapes[in.Op].result {
					defs = append(defs, in.Dst)
				}
			}
			for _, t := range blk.Term.Targets {
				live := c.liveIn[t]
				if len(live) == 0 {
					continue
				}
				for l := range c.definedIn[bid] {
					if live[l] && !c.definedIn[t][l] {
						c.definedIn[t][l] = true
						changed = true
					}
				}
				for _, l := range defs {
					if live[l] && !c.definedIn[t][l] {
						c.definedIn[t][l] = true
						changed = true
					}
				}
			}
		}
	}
}

func (c *borrowCheck) enterDefs(bid BlockID) {
	c.firstDef = map[LocalID]int{}
	for i, in := range c.f.Blocks[bid].Insts {
		if _, ok := c.holders[in.Dst]; ok && shapes[in.Op].result {
			if _, seen := c.firstDef[in.Dst]; !seen {
				c.firstDef[in.Dst] = i
			}
		}
	}
}

func (c *borrowCheck) definedLive(live map[LocalID]bool, bid BlockID, idx int) map[LocalID]bool {
	out := map[LocalID]bool{}
	for l := range live {
		if d, ok := c.firstDef[l]; c.definedIn[bid][l] || ok && d < idx {
			out[l] = true
		}
	}
	return out
}
