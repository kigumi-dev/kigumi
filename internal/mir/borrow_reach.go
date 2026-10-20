package mir

// borrowReach computes the owners borrowed on some path into each block: a widening
// container can be live before a borrow it will carry exists, unlike any other holder.
func (c *borrowCheck) borrowReach() {
	if len(c.sinks) == 0 {
		return
	}
	n := len(c.f.Blocks)
	c.borrowedIn = make([]map[LocalID]bool, n)
	for i := range n {
		c.borrowedIn[i] = map[LocalID]bool{}
	}
	for changed := true; changed; {
		changed = false
		for bid := range n {
			out := map[LocalID]bool{}
			for l := range c.borrowedIn[bid] {
				out[l] = true
			}
			for _, in := range c.f.Blocks[bid].Insts {
				if root, ok := c.borrowRoot(in); ok {
					out[root] = true
				}
			}
			for _, t := range c.f.Blocks[bid].Term.Targets {
				for l := range out {
					if !c.borrowedIn[t][l] {
						c.borrowedIn[t][l] = true
						changed = true
					}
				}
			}
		}
	}
}

func (c *borrowCheck) borrowRoot(in Inst) (LocalID, bool) {
	if in.Op != OpBorrow {
		return 0, false
	}
	root, _, _, ok := c.projectionPlace(in.Args[0])
	return root, ok
}

func (c *borrowCheck) enterBlock(bid BlockID) {
	if c.borrowedIn == nil {
		return
	}
	c.firstBorrow = map[LocalID]int{}
	for i, in := range c.f.Blocks[bid].Insts {
		if root, ok := c.borrowRoot(in); ok {
			if _, seen := c.firstBorrow[root]; !seen {
				c.firstBorrow[root] = i
			}
		}
	}
	c.curBlock = bid
}

func (c *borrowCheck) mayBeBorrowed(owner LocalID, at int) bool {
	if c.borrowedIn == nil || c.borrowedIn[c.curBlock][owner] {
		return true
	}
	i, ok := c.firstBorrow[owner]
	return ok && i < at
}
