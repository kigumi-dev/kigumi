package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
)

type tieState struct {
	sites  map[midir.LocalID][]carrySite
	in     map[[2]midir.LocalID][]bool
	caps   map[midir.LocalID]*borrowInfo
	stable map[midir.LocalID]bool
	// scalarCells are scalar field or element `&mut` cells a call's result would keep (E1122).
	scalarCells map[midir.LocalID]bool
}

// tiedAt gates the tie on the call's reach, after the write-backs of both the target and a `&mut` cell value, or of
// every argument for a kept flow, whose values may come from any of them; b stays unmarked when the site is not found.
func (c *borrowCheck) tiedAt(fl storeFlow, value, holder midir.LocalID, b *borrowInfo) *borrowInfo {
	idx, ok := c.tieSite(fl.target, holder)
	if !ok {
		return b
	}
	args := []midir.LocalID{value}
	if fl.kept {
		args = c.f.Blocks[c.resets.at.block].Insts[c.resets.at.idx].Args
	}
	idx = c.pastWriteBacks(idx, args)
	out := *b
	out.owners = slices.Clone(b.owners)
	for i := range out.owners {
		out.owners[i].tied = true
		c.addTieSite(holder, carrySite{block: c.resets.at.block, idx: idx, owner: out.owners[i].owner})
	}
	return &out
}

func (c *borrowCheck) pastWriteBacks(idx int, args []midir.LocalID) int {
	for _, a := range args {
		if wb, ok := c.cellWriteBack(a); ok {
			idx = max(idx, wb)
		}
	}
	return idx
}

// retie: holder takes from's value at instruction idx of the current block, with the tied owners from holds there.
func (c *borrowCheck) retie(holder midir.LocalID, idx int, from midir.LocalID, b *borrowInfo) {
	for _, o := range b.owners {
		if o.tied {
			c.addTieSite(holder, carrySite{block: c.resets.at.block, idx: idx, owner: o.owner, from: from})
		}
	}
}

func (c *borrowCheck) addTieSite(holder midir.LocalID, site carrySite) {
	if c.ties.sites == nil {
		c.ties.sites = map[midir.LocalID][]carrySite{}
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

// tieSite is the write-back of a `&mut` cell target (whole, field or element) when it follows in the call's block, else the call.
func (c *borrowCheck) tieSite(target, holder midir.LocalID) (int, bool) {
	at := c.resets.at
	if d := c.definition(target); d == nil || d.Op != midir.OpNewCell {
		return at.idx, true
	}
	insts := c.f.Blocks[at.block].Insts
	for i := at.idx + 1; i < len(insts); i++ {
		if wb, ok := c.writeBackOf(insts[i], holder); ok && wb.cell == target {
			return i, true
		}
	}
	// No write-back: the callee stores through the cell in place (a `mut self` field store), so the call is the site.
	return at.idx, true
}

func (c *borrowCheck) tieReaches(holder, owner midir.LocalID, at int) bool {
	return c.tieHeld(holder, owner, c.curBlock, at, len(c.f.Locals))
}

// tieHeld: a copy or a part holds a tie only if its source held it there, a call result or closure only if an operand did.
func (c *borrowCheck) tieHeld(holder, owner midir.LocalID, bid midir.BlockID, at, depth int) bool {
	sites := c.ties.sites[holder]
	if len(sites) == 0 {
		d, db, di, ok := c.soleDefAt(holder)
		switch {
		case !ok || depth == 0:
		case d.Op == midir.OpCopy || d.Op == midir.OpMove || d.Op == midir.OpAlias || d.Op == midir.OpField || d.Op == midir.OpFieldMove || d.Op == midir.OpPayload || d.Op == midir.OpIndex:
			return c.tieHeld(d.Args[0], owner, db, di, depth-1)
		case d.Op == midir.OpCall || d.Op == midir.OpCallValue || d.Op == midir.OpClosure:
			return c.argsHoldTie(d.Args, owner, db, di, depth-1)
		}
		return true
	}
	key := [2]midir.LocalID{holder, owner}
	in, ok := c.ties.in[key]
	if !ok {
		if c.ties.in == nil {
			c.ties.in = map[[2]midir.LocalID][]bool{}
		}
		// A cycle of copies reads this all-held placeholder while the answer is being computed.
		c.ties.in[key] = slices.Repeat([]bool{true}, len(c.f.Blocks))
		in = c.carryDataflow(holder, owner, sites, false)
		c.ties.in[key] = in
	}
	return c.carryStep(bid, holder, owner, sites, in[bid], at, false)
}

// argsHoldTie: the callee may return any operand, so its result holds owner where an operand carrying it does.
func (c *borrowCheck) argsHoldTie(args []midir.LocalID, owner midir.LocalID, bid midir.BlockID, at, depth int) bool {
	held, carried := c.operandsHoldTie(args, owner, bid, at, depth)
	return held || !carried
}

// operandsHoldTie judges a closure operand by its captures where it was made; a tie no capture accounts for stays held.
func (c *borrowCheck) operandsHoldTie(args []midir.LocalID, owner midir.LocalID, bid midir.BlockID, at, depth int) (held, carried bool) {
	for _, a := range args {
		tied, untied := c.carriesOwner(a, owner)
		if d, db, di, ok := c.soleDefAt(a); ok && d.Op == midir.OpClosure && depth > 0 {
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
func (c *borrowCheck) carriesOwner(a, owner midir.LocalID) (tied, untied bool) {
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

func (c *borrowCheck) soleDefAt(l midir.LocalID) (*midir.Inst, midir.BlockID, int, bool) {
	var def *midir.Inst
	var db midir.BlockID
	di := 0
	for b := range c.f.Blocks {
		for i := range c.f.Blocks[b].Insts {
			if in := &c.f.Blocks[b].Insts[i]; in.Op.Yields() && in.Dst == l {
				if def != nil {
					return nil, 0, 0, false
				}
				def, db, di = in, midir.BlockID(b), i
			}
		}
	}
	return def, db, di, def != nil && len(def.Args) > 0
}
