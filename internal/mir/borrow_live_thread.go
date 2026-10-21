package mir

import "kigumi/internal/sem"

// liveThrough is the holders live on entry to bid given those live at its exit.
func (c *borrowCheck) liveThrough(bid BlockID, out map[LocalID]bool, skip func(LocalID) bool) map[LocalID]bool {
	blk := &c.f.Blocks[bid]
	live := copySet(out)
	for _, a := range blk.Term.Args {
		if _, ok := c.holders[a]; ok {
			live[a] = true
		}
	}
	for i := len(blk.Insts) - 1; i >= 0; i-- {
		c.stepBack(blk.Insts[i], live, skip)
	}
	return live
}

// stepBack turns the holders live after in into those live before it; a drop of a holder skip accepts is no use.
func (c *borrowCheck) stepBack(in Inst, live map[LocalID]bool, skip func(LocalID) bool) {
	if _, ok := c.holders[in.Dst]; ok && in.Op != OpDrop && in.Op != OpSetField && in.Op != OpSetIndex && in.Op != OpCellSet && in.Op != OpGlobalSet && in.Op != OpFutureDone && in.Op != OpCancelDone {
		delete(live, in.Dst)
	}
	if in.Op == OpDrop && skip(in.Args[0]) {
		return
	}
	for _, a := range uses(in) {
		if _, ok := c.holders[a]; ok {
			live[a] = true
		}
	}
}

// plainRefRelease reports whether dropping l only releases a count on a Copy referent,
// which runs no drop code and so does not need the borrowed place.
func (c *borrowCheck) plainRefRelease(l LocalID) bool {
	return c.refRelease(l) && c.p.R.IsCopy(c.p.R.Types.Node(c.f.Locals[l].Type).Elem)
}

func (c *borrowCheck) refRelease(l LocalID) bool {
	return c.p.R.Types.Kind(c.f.Locals[l].Type) == sem.KRef
}

// mayDrop reports whether in may end or replace a counted value, which a pending release could outlive.
func (c *borrowCheck) mayDrop(in Inst) bool {
	switch in.Op {
	case OpMove, OpDrop, OpFieldMove, OpNewCell:
		return true
	case OpBorrow:
		return in.Index == 1
	case OpSetField, OpCellSet:
		return !c.p.R.IsCopy(c.f.Locals[in.Args[1]].Type)
	case OpSetIndex:
		return !c.p.R.IsCopy(c.f.Locals[in.Args[2]].Type)
	}
	return c.takesPayload(in)
}

// threadedTarget resolves the branch ending t when the edge from bid already decided its
// condition, as an `if v is P(x)` join re-tests the flag the test set.
func (c *borrowCheck) threadedTarget(bid, t BlockID) (BlockID, bool) {
	tt := c.f.Blocks[t].Term
	if tt.Op != TermBranch || len(tt.Args) != 1 || len(tt.Targets) != 2 || tt.Targets[0] == tt.Targets[1] {
		return 0, false
	}
	cond := tt.Args[0]
	val, ok := c.edgeCond(bid, t, cond)
	if !ok || c.definesIn(t, cond) {
		return 0, false
	}
	if val {
		return tt.Targets[0], true
	}
	return tt.Targets[1], true
}

// edgeCond is cond's value on the edge from bid to t: bid branches on it, or bid has one
// predecessor that does and bid does not assign it.
func (c *borrowCheck) edgeCond(bid, t BlockID, cond LocalID) (bool, bool) {
	if v, ok := branchValue(c.f.Blocks[bid].Term, t, cond); ok {
		return v, true
	}
	if c.f.Blocks[bid].Term.Op != TermJump || c.definesIn(bid, cond) {
		return false, false
	}
	pred, ok := c.solePred(bid)
	if !ok {
		return false, false
	}
	return branchValue(c.f.Blocks[pred].Term, bid, cond)
}

func branchValue(term Term, to BlockID, cond LocalID) (bool, bool) {
	if term.Op != TermBranch || len(term.Args) != 1 || term.Args[0] != cond || len(term.Targets) != 2 || term.Targets[0] == term.Targets[1] {
		return false, false
	}
	switch to {
	case term.Targets[0]:
		return true, true
	case term.Targets[1]:
		return false, true
	}
	return false, false
}

func (c *borrowCheck) definesIn(bid BlockID, l LocalID) bool {
	for _, in := range c.f.Blocks[bid].Insts {
		if shapes[in.Op].result && in.Dst == l {
			return true
		}
	}
	return false
}

func (c *borrowCheck) solePred(bid BlockID) (BlockID, bool) {
	if c.soleFrom == nil {
		c.soleFrom = map[BlockID]BlockID{}
		count := map[BlockID]int{}
		for i := range c.f.Blocks {
			for _, t := range c.f.Blocks[i].Term.Targets {
				count[t]++
				c.soleFrom[t] = BlockID(i)
			}
		}
		for t, n := range count {
			if n != 1 {
				delete(c.soleFrom, t)
			}
		}
	}
	p, ok := c.soleFrom[bid]
	return p, ok
}
