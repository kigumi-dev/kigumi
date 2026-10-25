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
	if c.ties.sites == nil {
		c.ties.sites = map[LocalID][]carrySite{}
	}
	for i := range out.owners {
		out.owners[i].tied = true
		site := carrySite{block: c.resets.at.block, idx: idx, owner: out.owners[i].owner}
		if !slices.Contains(c.ties.sites[holder], site) {
			c.ties.sites[holder] = append(c.ties.sites[holder], site)
		}
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

// tieHeld: a copy holds a tie only if its source held it where the copy was made.
func (c *borrowCheck) tieHeld(holder, owner LocalID, bid BlockID, at, depth int) bool {
	sites := c.ties.sites[holder]
	if len(sites) == 0 {
		if d, db, di, ok := c.soleDefAt(holder); ok && depth > 0 && (d.Op == OpCopy || d.Op == OpMove || d.Op == OpAlias) {
			return c.tieHeld(d.Args[0], owner, db, di, depth-1)
		}
		return true
	}
	key := [2]LocalID{holder, owner}
	in, ok := c.ties.in[key]
	if !ok {
		in = c.carryDataflow(holder, owner, sites, false)
		if c.ties.in == nil {
			c.ties.in = map[[2]LocalID][]bool{}
		}
		c.ties.in[key] = in
	}
	return c.carryStep(bid, holder, owner, sites, in[bid], at, false)
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
