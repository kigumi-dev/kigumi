package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// liveThrough is the holders live on entry to bid given those live at its exit.
func (c *borrowCheck) liveThrough(bid midir.BlockID, out map[midir.LocalID]bool, skip func(midir.LocalID) bool) map[midir.LocalID]bool {
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
func (c *borrowCheck) stepBack(in midir.Inst, live map[midir.LocalID]bool, skip func(midir.LocalID) bool) {
	if _, ok := c.holders[in.Dst]; ok && definesDst(in) {
		delete(live, in.Dst)
	}
	if in.Op == midir.OpDrop && (in.Emptied || skip(in.Args[0])) {
		return
	}
	for _, a := range uses(in) {
		if _, ok := c.holders[a]; ok {
			live[a] = true
		}
	}
}

// definesDst reports whether in gives Dst a new value rather than writing into or completing what it holds.
func definesDst(in midir.Inst) bool {
	switch in.Op {
	case midir.OpDrop, midir.OpSetField, midir.OpSetIndex, midir.OpCellSet, midir.OpGlobalSet, midir.OpFutureDone, midir.OpCancelDone:
		return false
	}
	return true
}

// plainRefRelease reports whether dropping l only releases a count on a Copy referent,
// which runs no drop code and so does not need the borrowed place.
func (c *borrowCheck) plainRefRelease(l midir.LocalID) bool {
	return c.refRelease(l) && c.p.R.IsCopy(c.p.R.Types.Node(c.f.Locals[l].Type).Elem)
}

func (c *borrowCheck) refRelease(l midir.LocalID) bool {
	return c.p.R.Types.Kind(c.f.Locals[l].Type) == sem.KRef || c.releaseOnly(l) || c.inertDrop(l)
}

// mayDrop reports whether in may end or replace a counted value, which a pending release could outlive.
func (c *borrowCheck) mayDrop(in midir.Inst) bool {
	switch in.Op {
	case midir.OpMove, midir.OpDrop, midir.OpFieldMove, midir.OpNewCell:
		return true
	case midir.OpBorrow:
		return in.Index == 1
	case midir.OpSetField, midir.OpCellSet:
		return !c.p.R.IsCopy(c.f.Locals[in.Args[1]].Type)
	case midir.OpSetIndex:
		return !c.p.R.IsCopy(c.f.Locals[in.Args[2]].Type)
	}
	return c.takesPayload(in)
}

func (c *borrowCheck) threadedTarget(bid, t midir.BlockID) (midir.BlockID, bool) {
	if c.thread == nil {
		c.thread = &flagThread{f: c.f}
	}
	return c.thread.target(bid, t)
}
