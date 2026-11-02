package borrow

import (
	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
)

// endedOwner is the owner a borrow of arg, a place under ref's root, depends on: the root while it lives, else the
// copy arg read out of it before its end, which lasts only as long as the full expression that made it.
func (c *borrowCheck) endedOwner(ref ownerRef, arg midir.LocalID) ownerRef {
	if !c.endedAt(ref.owner) {
		return ref
	}
	if c.f.Locals[arg].Ent == 0 {
		ref = ownerRef{owner: arg, mut: ref.mut, exact: true}
	}
	ref.temp = true
	return ref
}

// endedAt reports whether root may already be gone at the instruction being collected: on some path to it a
// drop or whole move of root follows its last definition, so a borrow there holds only a statement's copy.
func (c *borrowCheck) endedAt(root midir.LocalID) bool {
	at := c.resets.at
	ended := c.endedIn(root)[at.block]
	for _, in := range c.f.Blocks[at.block].Insts[:at.idx] {
		ended = endStep(in, root, ended)
	}
	return ended
}

func endStep(in midir.Inst, root midir.LocalID, ended bool) bool {
	switch {
	case (in.Op == midir.OpDrop || in.Op == midir.OpMove) && len(in.Args) > 0 && in.Args[0] == root:
		return true
	case in.Op.Yields() && in.Dst == root:
		return false
	}
	return ended
}

// endedIn is, per block, whether root may be ended on entry.
func (c *borrowCheck) endedIn(root midir.LocalID) []bool {
	if s, ok := c.ended[root]; ok {
		return s
	}
	if c.ended == nil {
		c.ended = map[midir.LocalID][]bool{}
	}
	entry := make([]bool, len(c.f.Blocks))
	order := mirflow.ReversePostorder(c.f)
	for changed := true; changed; {
		changed = false
		for _, b := range order {
			ended := entry[b]
			for _, in := range c.f.Blocks[b].Insts {
				ended = endStep(in, root, ended)
			}
			for _, t := range c.f.Blocks[b].Term.Targets {
				if ended && !entry[t] {
					entry[t], changed = true, true
				}
			}
		}
	}
	c.ended[root] = entry
	return entry
}
