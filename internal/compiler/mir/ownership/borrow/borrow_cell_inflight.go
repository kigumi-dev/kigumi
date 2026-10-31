package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// checkOffloads reports a touch of a cell place a closure still running on another thread reaches: while a value that
// can hold the Future or Task its hand-off returned is live, or for the rest of the body once the task scope holds it.
func (c *borrowCheck) checkOffloads() {
	if !c.offloadPossible() || !c.offloadsHere() {
		return
	}
	c.offloadNode = map[sem.EntityID]syntax.NodeID{}
	fl := c.solveFlow(c.offloadSeed, c.offloadStep)
	handles := map[midir.LocalID]bool{}
	for l, fs := range fl.ever {
		if l != c.spawnScope() && handsOff(fs) && c.holdsDeferred(c.f.Locals[l].Type) {
			handles[l] = true
		}
	}
	if len(handles) == 0 && !handsOff(fl.ever[c.spawnScope()]) {
		return
	}
	cells := map[sem.EntityID][]midir.LocalID{}
	for l, loc := range c.f.Locals {
		if loc.Cell && loc.Ent != 0 {
			cells[loc.Ent] = append(cells[loc.Ent], midir.LocalID(l))
		}
	}
	same := c.sameCellSince(fl)
	h := handleSet{is: handles, empty: c.emptyHandles(handles)}
	liveOut := c.handleLiveOut(h)
	for bid := range c.f.Blocks {
		c.enterBlock(midir.BlockID(bid))
		after := make([][]midir.LocalID, len(c.f.Blocks[bid].Insts))
		c.handleWalk(midir.BlockID(bid), liveOut[bid], h, func(i int, live map[midir.LocalID]bool) {
			for l := range live {
				after[i] = append(after[i], l)
			}
		})
		for i, in := range c.f.Blocks[bid].Insts {
			c.curIdx = i
			cls := c.inFlight(fl, append(after[i], c.spawnScope()))
			// The task scope lists an in-flight closure only because the await or spawn that took its handle put it there.
			handed := func(a midir.LocalID, cl sem.EntityID) bool {
				return c.flowHas(fl, a, uint32(cl)|offBit) || a == c.spawnScope() && slices.Contains(cls, cl)
			}
			for _, cl := range cls {
				c.checkInFlight(in, cl, cells, handed, same)
			}
		}
	}
}

func handsOff(fs map[uint32]bool) bool {
	for f := range fs {
		if f&offBit != 0 {
			return true
		}
	}
	return false
}

// offloadsHere reports whether this body can hand work to another thread: a call that offloads, or a closure or async
// fn built here whose run may.
func (c *borrowCheck) offloadsHere() bool {
	for _, blk := range c.f.Blocks {
		for _, in := range blk.Insts {
			switch {
			case (in.Op == midir.OpCall || in.Op == midir.OpCallValue) && c.offloadsAt(in):
				return true
			case in.Op == midir.OpClosure && c.p.R.Entity(in.Ent).Kind == sem.EntClosure, in.Op == midir.OpFnItem && in.Async:
				if c.mayOffload(in.Ent) {
					return true
				}
			}
		}
	}
	return false
}

// inFlight are the closures handed off that the given values, live after the instruction being checked, keep running.
func (c *borrowCheck) inFlight(fl *valueFlow, live []midir.LocalID) []sem.EntityID {
	var out []sem.EntityID
	for _, l := range live {
		for f := range c.flowAt(fl, l) {
			if cl := sem.EntityID(f &^ offBit); f&offBit != 0 && !slices.Contains(out, cl) {
				out = append(out, cl)
			}
		}
	}
	slices.Sort(out)
	return out
}

// checkInFlight reports in touching a cell place in-flight closure cl reaches, or running another closure that does,
// when either writes it; releasing the cell itself leaves its content to the closure.
func (c *borrowCheck) checkInFlight(in midir.Inst, cl sem.EntityID, cells map[sem.EntityID][]midir.LocalID, handed func(midir.LocalID, sem.EntityID) bool, same func(sem.EntityID, midir.LocalID) bool) {
	for i, cp := range c.p.R.Closure(cl).Captures {
		if cp.Mode != sem.CapCell {
			continue
		}
		for _, owner := range cells[cp.Local] {
			if !same(cl, owner) {
				continue
			}
			hit, writes := false, false
			for _, u := range c.closureUses(cl, i) {
				o := ownerRef{owner: owner, mut: u.writes, path: u.path}
				if h, _ := c.cellRunHitExcept(in, o, handed); h || c.touchesInFlight(in, o) {
					hit, writes = true, writes || u.writes
				}
			}
			if hit {
				c.reportInFlight(in, owner, cl, writes)
			}
		}
	}
}

func (c *borrowCheck) touchesInFlight(in midir.Inst, o ownerRef) bool {
	if in.Op == midir.OpDrop && in.Args[0] == o.owner {
		return false
	}
	touched, path, _ := c.touch(in, o.owner)
	return touched && ownquery.FieldPathsOverlap(path, o.path) && (o.mut || c.writesOwner(in, o.owner) || in.Op.Yields() && in.Dst == o.owner)
}

func (c *borrowCheck) reportInFlight(in midir.Inst, owner midir.LocalID, cl sem.EntityID, writes bool) {
	name := "`" + c.f.Locals[owner].Name + "`"
	msg := name + " is read by a closure that may still be running on another thread, and cannot be mutated here"
	if writes {
		msg = name + " is written by a closure that may still be running on another thread, and cannot be used here"
	}
	c.reportHelped(in.Node, owner, "borrow-conflict", msg, c.offloadNode[cl], "the closure is handed off here",
		"await the result of the hand-off before using "+name+", or give the closure a copy of its own")
}
