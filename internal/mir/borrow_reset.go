package mir

// resetState tracks the whole assignments of a named holder, so a borrow it held stops counting once the holder is reassigned.
type resetState struct {
	at      carrySite
	sites   map[LocalID][]carrySite
	tracked map[LocalID]bool
	in      map[[2]LocalID][]bool
	stored  map[LocalID]bool
	ends    map[carrySite]bool
}

func (c *borrowCheck) noteResetSite(dst LocalID, b *borrowInfo) {
	if c.f.Locals[dst].Ent == 0 {
		return
	}
	if c.resets.sites == nil {
		c.resets.sites = map[LocalID][]carrySite{}
	}
	for _, o := range b.owners {
		site := carrySite{block: c.resets.at.block, idx: c.resets.at.idx, owner: o.owner}
		known := false
		for _, s := range c.resets.sites[dst] {
			known = known || s == site
		}
		if !known {
			c.resets.sites[dst] = append(c.resets.sites[dst], site)
		}
	}
}

// resetReaches reports whether a borrow of owner that holder took may still be held at instruction at of the current block.
func (c *borrowCheck) resetReaches(holder, owner LocalID, at int) bool {
	sites := c.resets.sites[holder]
	if len(sites) == 0 || !c.resetTracked(holder) {
		return true
	}
	key := [2]LocalID{holder, owner}
	in, ok := c.resets.in[key]
	if !ok {
		in = c.carryDataflow(holder, owner, sites, true)
		if c.resets.in == nil {
			c.resets.in = map[[2]LocalID][]bool{}
		}
		c.resets.in[key] = in
	}
	return c.carryStep(c.curBlock, holder, owner, sites, in[c.curBlock], at, true)
}

// resetTracked holds for a named local nothing stores into and whose every definition widens it at a recorded site.
func (c *borrowCheck) resetTracked(holder LocalID) bool {
	if t, ok := c.resets.tracked[holder]; ok {
		return t
	}
	ok := (!c.targets[holder] || c.storesOnlyReplace(holder)) && c.widens(holder)
	for _, p := range c.f.Params {
		ok = ok && p != holder
	}
	for _, blk := range c.f.Blocks {
		for _, in := range blk.Insts {
			if ok && shapes[in.Op].result && in.Dst == holder && !resetDef[in.Op] {
				ok = false
			}
		}
	}
	if c.resets.tracked == nil {
		c.resets.tracked = map[LocalID]bool{}
	}
	c.resets.tracked[holder] = ok
	return ok
}

var resetDef = map[Opcode]bool{OpCopy: true, OpMove: true, OpRecord: true, OpVariant: true, OpCall: true, OpCallValue: true, OpFutureNew: true}
