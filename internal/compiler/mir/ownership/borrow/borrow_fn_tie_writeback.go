package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// writeBackKeeps: insts[i] writes a `&mut` cell back into holder (or drops holder just before) and the call may not have stored.
func (c *borrowCheck) writeBackKeeps(bid midir.BlockID, i int, holder midir.LocalID) bool {
	insts := c.f.Blocks[bid].Insts
	if insts[i].Op == midir.OpDrop && insts[i].Scoped && i+1 < len(insts) {
		i++
	}
	key := carrySite{block: bid, idx: i, owner: holder}
	if keep, ok := c.writeBacks[key]; ok {
		return keep
	}
	wb, ok := c.writeBackOf(insts[i], holder)
	keep := ok && wb.kind == wbWhole && !c.callStores(insts[:i], wb.cell)
	if c.writeBacks == nil {
		c.writeBacks = map[carrySite]bool{}
	}
	c.writeBacks[key] = keep
	return keep
}

// callStores reports whether the last call before the write-back that was given cell stores into it on every path.
func (c *borrowCheck) callStores(before []midir.Inst, cell midir.LocalID) bool {
	for j := len(before) - 1; j >= 0; j-- {
		in := before[j]
		k := slices.Index(in.Args, cell)
		switch {
		case in.Op == midir.OpNewCell && in.Dst == cell:
			return false
		case k < 0 || in.Op != midir.OpCall && in.Op != midir.OpCallValue:
			continue
		case in.Op == midir.OpCallValue || in.Ent == 0 || c.p.R.Entity(in.Ent).Kind != sem.EntFn:
			return false
		}
		return c.analysis().storesParam(in.Ent, k)
	}
	return false
}

type paramSlot struct {
	fn sem.EntityID
	k  int
}

// storesParam reports whether fn replaces what its `&mut` parameter k holds on every path to return; a cycle of forwards counts as no.
func (p *Analyzer) storesParam(fn sem.EntityID, k int) bool {
	key := paramSlot{fn: fn, k: k}
	if s, ok := p.paramStores[key]; ok {
		return s
	}
	if p.paramStores == nil {
		p.paramStores = map[paramSlot]bool{}
	}
	p.paramStores[key] = false
	f, ok := p.ByEnt[fn]
	s := ok && p.R.Entity(fn).Kind == sem.EntFn && k < len(f.Params) && p.storedOnEveryPath(f, f.Params[k])
	p.paramStores[key] = s
	return s
}

// storedOnEveryPath: every return of f follows a `cellset` of cell, a fieldStore or a call that stores into it, and f never reads it.
func (p *Analyzer) storedOnEveryPath(f *midir.Func, cell midir.LocalID) bool {
	stored := make([]bool, len(f.Blocks))
	for b, blk := range f.Blocks {
		for _, in := range blk.Insts {
			for k, a := range in.Args {
				if a != cell || in.Op == midir.OpDrop || k == 0 && p.replacingUse(f, in, cell) {
					continue
				}
				if in.Op != midir.OpCall || in.Ent == 0 || !p.storesParam(in.Ent, k) {
					return false
				}
				stored[b] = true
			}
			stored[b] = stored[b] || in.Op == midir.OpCellSet && in.Args[0] == cell || p.fieldStore(f, in, cell)
		}
		if slices.Contains(blk.Term.Args, cell) {
			return false
		}
	}
	in := make([]bool, len(f.Blocks))
	for b := 1; b < len(in); b++ {
		in[b] = true
	}
	for changed := true; changed; {
		changed = false
		for b, blk := range f.Blocks {
			for _, t := range blk.Term.Targets {
				if in[t] && !in[b] && !stored[b] {
					in[t], changed = false, true
				}
			}
		}
	}
	for b, blk := range f.Blocks {
		if blk.Term.Op == midir.TermReturn && !in[b] && !stored[b] {
			return false
		}
	}
	return true
}
