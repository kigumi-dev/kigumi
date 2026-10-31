package flow

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// FlagThread resolves a branch that re-tests a flag an earlier branch already decided.
type FlagThread struct {
	f        *midir.Func
	soleFrom map[midir.BlockID]midir.BlockID
	preds    map[midir.BlockID][]midir.BlockID
}

// allThreaded reports whether every edge into t decides the flag t re-tests, so the state
// joined at t never reaches t's targets as such.
func (c *FlagThread) AllThreaded(t midir.BlockID) bool {
	if c.preds == nil {
		c.preds = map[midir.BlockID][]midir.BlockID{}
		for i := range c.f.Blocks {
			for _, s := range c.f.Blocks[i].Term.Targets {
				c.preds[s] = append(c.preds[s], midir.BlockID(i))
			}
		}
	}
	if len(c.preds[t]) == 0 {
		return false
	}
	for _, p := range c.preds[t] {
		if _, ok := c.Target(p, t); !ok {
			return false
		}
	}
	return true
}

// target resolves the branch ending t when the edge from bid already decided its
// condition, as an `if v is P(x)` join re-tests the flag the test set.
func (c *FlagThread) Target(bid, t midir.BlockID) (midir.BlockID, bool) {
	tt := c.f.Blocks[t].Term
	if tt.Op != midir.TermBranch || len(tt.Args) != 1 || len(tt.Targets) != 2 || tt.Targets[0] == tt.Targets[1] {
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
func (c *FlagThread) edgeCond(bid, t midir.BlockID, cond midir.LocalID) (bool, bool) {
	if v, ok := branchValue(c.f.Blocks[bid].Term, t, cond); ok {
		return v, true
	}
	if c.f.Blocks[bid].Term.Op != midir.TermJump || c.definesIn(bid, cond) {
		return false, false
	}
	pred, ok := c.solePred(bid)
	if !ok {
		return false, false
	}
	return branchValue(c.f.Blocks[pred].Term, bid, cond)
}

func branchValue(term midir.Term, to midir.BlockID, cond midir.LocalID) (bool, bool) {
	if term.Op != midir.TermBranch || len(term.Args) != 1 || term.Args[0] != cond || len(term.Targets) != 2 || term.Targets[0] == term.Targets[1] {
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

func (c *FlagThread) definesIn(bid midir.BlockID, l midir.LocalID) bool {
	for _, in := range c.f.Blocks[bid].Insts {
		if in.Op.Yields() && in.Dst == l {
			return true
		}
	}
	return false
}

func (c *FlagThread) solePred(bid midir.BlockID) (midir.BlockID, bool) {
	if c.soleFrom == nil {
		c.soleFrom = map[midir.BlockID]midir.BlockID{}
		count := map[midir.BlockID]int{}
		for i := range c.f.Blocks {
			for _, t := range c.f.Blocks[i].Term.Targets {
				count[t]++
				c.soleFrom[t] = midir.BlockID(i)
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

func NewFlagThread(f *midir.Func) *FlagThread { return &FlagThread{f: f} }
