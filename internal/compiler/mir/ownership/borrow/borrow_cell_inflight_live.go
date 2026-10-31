package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

type cellSinceKey struct {
	cl   sem.EntityID
	cell midir.LocalID
}

// sameCellSince answers whether in-flight closure cl may still share cell with this body at the instruction being
// checked: a hand-off of it, new to its block or carried round a loop, reaches here without cell being made anew.
func (c *borrowCheck) sameCellSince(fl *valueFlow) func(sem.EntityID, midir.LocalID) bool {
	sites := map[sem.EntityID][][2]int{}
	for bid := range c.f.Blocks {
		i := -1
		rec := *fl
		// replayFlow seeds each instruction once before its step, so the seed hook keeps the step's index.
		rec.seed = func(in midir.Inst) []uint32 { i++; return fl.seed(in) }
		rec.step = func(in midir.Inst, s flowSets, put func(midir.LocalID, uint32)) {
			fl.step(in, s, func(l midir.LocalID, f uint32) {
				if f&offBit != 0 {
					cl := sem.EntityID(f &^ offBit)
					sites[cl] = append(sites[cl], [2]int{bid, i + 1})
				}
				put(l, f)
			})
		}
		c.replayFlow(&rec, cloneFlowSets(fl.in[bid]), midir.BlockID(bid), nil)
	}
	memo := map[cellSinceKey]map[midir.BlockID][]int{}
	return func(cl sem.EntityID, cell midir.LocalID) bool {
		key := cellSinceKey{cl, cell}
		starts, ok := memo[key]
		if !ok {
			starts = c.reachUnlessRedefined(sites[cl], cell)
			memo[key] = starts
		}
		return slices.ContainsFunc(starts[c.curBlock], func(s int) bool { return s <= c.curIdx && !c.redefinedIn(c.curBlock, s, c.curIdx, cell) })
	}
}

// reachUnlessRedefined lists, per block, the indices control enters it at from the given points without passing a
// redefinition of cell.
func (c *borrowCheck) reachUnlessRedefined(from [][2]int, cell midir.LocalID) map[midir.BlockID][]int {
	starts := map[midir.BlockID][]int{}
	var work [][2]int
	push := func(b midir.BlockID, i int) {
		if !slices.Contains(starts[b], i) {
			starts[b] = append(starts[b], i)
			work = append(work, [2]int{int(b), i})
		}
	}
	for _, p := range from {
		push(midir.BlockID(p[0]), p[1])
	}
	for len(work) > 0 {
		b, i := midir.BlockID(work[len(work)-1][0]), work[len(work)-1][1]
		work = work[:len(work)-1]
		if !c.redefinedIn(b, i, len(c.f.Blocks[b].Insts)-1, cell) {
			for _, t := range c.f.Blocks[b].Term.Targets {
				push(t, 0)
			}
		}
	}
	return starts
}

func (c *borrowCheck) redefinedIn(b midir.BlockID, from, to int, cell midir.LocalID) bool {
	insts := c.f.Blocks[b].Insts
	for i := from; i <= to && i < len(insts); i++ {
		if insts[i].Op.Yields() && definesDst(insts[i]) && insts[i].Dst == cell {
			return true
		}
	}
	return false
}

// handleSet is the locals that may keep handed-off work running, with where each is known to hold nothing.
type handleSet struct {
	is    map[midir.LocalID]bool
	empty [][]map[midir.LocalID]bool
}

// heldAt reports whether l, used before instruction i of b (len(Insts) for the terminator), may still hold the work.
func (h handleSet) heldAt(b midir.BlockID, i int, l midir.LocalID) bool {
	return h.is[l] && !h.empty[b][i][l]
}

// handleLiveOut is, per block, the handles live at its end.
func (c *borrowCheck) handleLiveOut(h handleSet) []map[midir.LocalID]bool {
	n := len(c.f.Blocks)
	out, in := make([]map[midir.LocalID]bool, n), make([]map[midir.LocalID]bool, n)
	for changed := true; changed; {
		changed = false
		for bid := n - 1; bid >= 0; bid-- {
			o := map[midir.LocalID]bool{}
			for _, t := range c.f.Blocks[bid].Term.Targets {
				for l := range in[t] {
					o[l] = true
				}
			}
			live := c.handleWalk(midir.BlockID(bid), o, h, nil)
			if !sameSet(o, out[bid]) || !sameSet(live, in[bid]) {
				changed = true
			}
			out[bid], in[bid] = o, live
		}
	}
	return out
}

// handleWalk steps a block backward from the handles live at its end, passing each instruction the handles live after
// it: a handle is live until its last use while it may hold the work, which may be the drop that waits for it.
func (c *borrowCheck) handleWalk(bid midir.BlockID, out map[midir.LocalID]bool, h handleSet, after func(int, map[midir.LocalID]bool)) map[midir.LocalID]bool {
	blk := &c.f.Blocks[bid]
	live := copySet(out)
	for _, a := range blk.Term.Args {
		if h.heldAt(bid, len(blk.Insts), a) {
			live[a] = true
		}
	}
	for i := len(blk.Insts) - 1; i >= 0; i-- {
		in := blk.Insts[i]
		if after != nil {
			after(i, live)
		}
		if in.Op.Yields() && definesDst(in) {
			delete(live, in.Dst)
		}
		if in.Op == midir.OpDrop && in.Emptied {
			continue
		}
		for _, a := range in.Args {
			if h.heldAt(bid, i, a) {
				live[a] = true
			}
		}
	}
	return live
}
