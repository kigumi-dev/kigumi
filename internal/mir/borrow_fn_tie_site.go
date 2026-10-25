package mir

import "slices"

type tieState struct {
	sites map[LocalID][]carrySite
	in    map[[2]LocalID][]bool
	caps  map[LocalID]*borrowInfo
}

// tiedAt gates the tie on the call's reach; b stays unmarked when the site is not found.
func (c *borrowCheck) tiedAt(target, holder LocalID, b *borrowInfo) *borrowInfo {
	idx, ok := c.tieSite(target, holder)
	if !ok {
		return b
	}
	out := *b
	out.owners = slices.Clone(b.owners)
	for i := range out.owners {
		out.owners[i].tied = true
		c.addTieSite(holder, carrySite{block: c.resets.at.block, idx: idx, owner: out.owners[i].owner})
	}
	return &out
}

// retie: holder takes from's value at instruction idx of the current block, with the tied owners from holds there.
func (c *borrowCheck) retie(holder LocalID, idx int, from LocalID, b *borrowInfo) {
	for _, o := range b.owners {
		if o.tied {
			c.addTieSite(holder, carrySite{block: c.resets.at.block, idx: idx, owner: o.owner, from: from})
		}
	}
}

func (c *borrowCheck) addTieSite(holder LocalID, site carrySite) {
	if c.ties.sites == nil {
		c.ties.sites = map[LocalID][]carrySite{}
	}
	if !slices.Contains(c.ties.sites[holder], site) {
		c.ties.sites[holder] = append(c.ties.sites[holder], site)
	}
}

func untied(b *borrowInfo) *borrowInfo {
	out := *b
	out.owners = slices.Clone(b.owners)
	for i := range out.owners {
		out.owners[i].tied = false
	}
	return &out
}

// tieSite is the write-back of a `&mut` cell target when it follows in the call's block, else the call itself.
func (c *borrowCheck) tieSite(target, holder LocalID) (int, bool) {
	at := c.resets.at
	if d := c.definition(target); d == nil || d.Op != OpNewCell {
		return at.idx, true
	}
	insts := c.f.Blocks[at.block].Insts
	for i := at.idx + 1; i < len(insts); i++ {
		if in := insts[i]; in.Dst == holder && (in.Op == OpCopy || in.Op == OpMove) && len(in.Args) > 0 {
			if d := c.definition(in.Args[0]); d != nil && d.Op == OpCellTake && d.Args[0] == target {
				return i, true
			}
		}
	}
	return 0, false
}

func (c *borrowCheck) tieReaches(holder, owner LocalID, at int) bool {
	return c.tieHeld(holder, owner, c.curBlock, at, len(c.f.Locals))
}

// tieHeld: a copy or a part holds a tie only if its source held it there, a call result or closure only if an operand did.
func (c *borrowCheck) tieHeld(holder, owner LocalID, bid BlockID, at, depth int) bool {
	sites := c.ties.sites[holder]
	if len(sites) == 0 {
		d, db, di, ok := c.soleDefAt(holder)
		switch {
		case !ok || depth == 0:
		case d.Op == OpCopy || d.Op == OpMove || d.Op == OpAlias || d.Op == OpField || d.Op == OpFieldMove || d.Op == OpPayload || d.Op == OpIndex:
			return c.tieHeld(d.Args[0], owner, db, di, depth-1)
		case d.Op == OpCall || d.Op == OpCallValue || d.Op == OpClosure:
			return c.argsHoldTie(d.Args, owner, db, di, depth-1)
		}
		return true
	}
	key := [2]LocalID{holder, owner}
	in, ok := c.ties.in[key]
	if !ok {
		if c.ties.in == nil {
			c.ties.in = map[[2]LocalID][]bool{}
		}
		// A cycle of copies reads this all-held placeholder while the answer is being computed.
		c.ties.in[key] = slices.Repeat([]bool{true}, len(c.f.Blocks))
		in = c.carryDataflow(holder, owner, sites, false)
		c.ties.in[key] = in
	}
	return c.carryStep(bid, holder, owner, sites, in[bid], at, false)
}

// argsHoldTie: the callee may return any operand, so its result holds owner where an operand carrying it does.
func (c *borrowCheck) argsHoldTie(args []LocalID, owner LocalID, bid BlockID, at, depth int) bool {
	held, carried := c.operandsHoldTie(args, owner, bid, at, depth)
	return held || !carried
}

// operandsHoldTie judges a closure operand by its captures where it was made; a tie no capture accounts for stays held.
func (c *borrowCheck) operandsHoldTie(args []LocalID, owner LocalID, bid BlockID, at, depth int) (held, carried bool) {
	for _, a := range args {
		tied, untied := c.carriesOwner(a, owner)
		if d, db, di, ok := c.soleDefAt(a); ok && d.Op == OpClosure && depth > 0 {
			h, k := c.operandsHoldTie(d.Args, owner, db, di, depth-1)
			if untied || h || tied && !k {
				return true, true
			}
			carried = carried || tied || k
			continue
		}
		if untied || tied && c.tieHeld(a, owner, bid, at, depth) {
			return true, true
		}
		carried = carried || tied
	}
	return false, carried
}

// carriesOwner reports whether a's borrows, or the captured borrows its fn value returns, include owner tied or untied.
func (c *borrowCheck) carriesOwner(a, owner LocalID) (tied, untied bool) {
	for _, b := range []*borrowInfo{c.holders[a], c.ties.caps[a]} {
		if b == nil {
			continue
		}
		for _, o := range b.owners {
			if o.owner == owner {
				tied, untied = tied || o.tied, untied || !o.tied
			}
		}
	}
	return tied, untied
}

func (c *borrowCheck) soleDefAt(l LocalID) (*Inst, BlockID, int, bool) {
	var def *Inst
	var db BlockID
	di := 0
	for b := range c.f.Blocks {
		for i := range c.f.Blocks[b].Insts {
			if in := &c.f.Blocks[b].Insts[i]; shapes[in.Op].result && in.Dst == l {
				if def != nil {
					return nil, 0, 0, false
				}
				def, db, di = in, BlockID(b), i
			}
		}
	}
	return def, db, di, def != nil && len(def.Args) > 0
}
