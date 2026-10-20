package mir

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// Borrow rules, checked per function on the MIR: a borrow may not
// escape (return, store, capture), the owner cannot be touched while an
// exclusive borrow is live, and cannot be mutated, moved or dropped while
// a shared borrow is live. Liveness is by last use.

// ownerRef is one local a borrow may actually alias, with the field chain
// (see projectionPlace) it was taken through.
type ownerRef struct {
	owner LocalID
	mut   bool
	path  []int
	exact bool
}

// borrowInfo tracks every owner a holder might alias, not just one: an ambiguous
// same-kind call result (sameKindCandidate) can't name its single true source.
type borrowInfo struct {
	owners   []ownerRef
	hasOwner bool
	node     syntax.NodeID
	// lifetime is the named lifetime a param-inherited holder (hasOwner false)
	// traces back to, 0 when it cannot be named.
	lifetime sem.EntityID
	// fromStatic marks a holder traced to a static's storage, so
	// checkLifetimeEscape can report it with its own wording.
	fromStatic bool
	empty      bool
	// multi is the type naming several lifetimes that left lifetime unnamed.
	multi sem.TypeID
}

type borrowCheck struct {
	p       *Program
	f       *Func
	holders map[LocalID]*borrowInfo
	liveOut []map[LocalID]bool
	// definedIn holds the holders that may have been assigned on some
	// path into a block: a binding that `is` assigns only when its pattern
	// matches is live at the join but not yet defined before the test.
	definedIn []map[LocalID]bool
	reported  map[reportKey]bool
	quiet     bool
	// defs maps a temporary to its defining instruction (nil when defined
	// more than once), built on first use.
	defs map[LocalID]*Inst
	// chainOnly caches chainedOnly's result per local.
	chainOnly map[LocalID]bool
	// unowned caches unownedRefs, built on first use.
	unowned []bool
	// sinks are the holders collect re-merges on every round: containers
	// whose element sources are the union of every write.
	sinks       map[LocalID]*borrowInfo
	targets     map[LocalID]bool
	contents    map[LocalID]*borrowInfo
	caps        map[LocalID]*borrowInfo
	grew        bool
	borrowedIn  []map[LocalID]bool
	firstBorrow map[LocalID]int
	curBlock    BlockID
	curIdx      int
}

func analyzeBorrows(p *Program, f *Func) {
	c := &borrowCheck{p: p, f: f, holders: map[LocalID]*borrowInfo{}, reported: map[reportKey]bool{}}
	c.collect()
	if len(c.holders) == 0 {
		return
	}
	c.liveness()
	c.definitions()
	c.borrowReach()
	for bid := range f.Blocks {
		c.checkBlock(BlockID(bid))
	}
}

// checkBlock walks a block backward, tracking the holders live after each
// instruction, and reports the conflicts and escapes.
func (c *borrowCheck) checkBlock(bid BlockID) {
	blk := &c.f.Blocks[bid]
	live := map[LocalID]bool{}
	for l := range c.liveOut[bid] {
		live[l] = true
	}
	for _, a := range blk.Term.Args {
		if b, ok := c.holders[a]; ok {
			// A holder with no owner is a borrow inherited from a
			// parameter, not one taken of a local: returning it is the
			// intended lifetime escape, and the caller's
			// own call site becomes the new holder (lifetimeSource).
			if !b.hasOwner {
				c.checkLifetimeEscape(a, b)
				live[a] = true
				continue
			}
			c.report(b.node, a, "borrow-escape", "a borrow cannot be returned")
			live[a] = true
		}
	}
	before := c.definedBefore(bid)
	c.enterBlock(bid)
	for i := len(blk.Insts) - 1; i >= 0; i-- {
		in := blk.Insts[i]
		c.curIdx = i
		c.checkInst(in, intersect(live, before[i]))
		if _, ok := c.holders[in.Dst]; ok && in.Op != OpDrop && in.Op != OpSetField && in.Op != OpSetIndex && in.Op != OpCellSet && in.Op != OpGlobalSet && in.Op != OpFutureDone && in.Op != OpCancelDone {
			delete(live, in.Dst)
		}
		for _, a := range uses(in) {
			if _, ok := c.holders[a]; ok {
				live[a] = true
			}
		}
	}
}

// checkInst sees the holders live after in; a conflict is a touch of an
// owner whose borrow outlives this instruction.
func (c *borrowCheck) checkInst(in Inst, liveAfter map[LocalID]bool) {
	c.checkCapture(in)
	c.checkSuspend(in, liveAfter)
	// A receiver cell's write-back releases the place's old value; the
	// exclusive borrow the call took of that place already reported.
	if in.Op == OpDrop && in.Scoped {
		return
	}
	for holder := range liveAfter {
		b := c.holders[holder]
		if !b.hasOwner || holder == in.Dst {
			continue
		}
		for _, o := range b.owners {
			if !c.mayBeBorrowed(o.owner, c.curIdx) {
				continue
			}
			touched, path, exact := c.touch(in, o.owner)
			if !touched || !fieldPathsOverlap(path, o.path, exact, o.exact) {
				continue
			}
			mutates := in.Op == OpMove || in.Op == OpDrop || (in.Op == OpBorrow && in.Index == 1) ||
				in.Op == OpSetField || in.Op == OpSetIndex || in.Op == OpCellSet
			name := c.f.Locals[o.owner].Name
			switch {
			case in.Op == OpDrop && in.EndScope:
				c.report(in.Node, o.owner, "move-while-borrowed", "`"+name+"` is still borrowed when its scope ends here")
			case in.Op == OpMove || in.Op == OpDrop:
				c.report(in.Node, o.owner, "move-while-borrowed", "`"+name+"` is borrowed and cannot be moved or dropped here")
			case o.mut:
				c.report(in.Node, o.owner, "borrow-conflict", "`"+name+"` is exclusively borrowed and cannot be used here")
			case mutates || in.Dst == o.owner:
				c.report(in.Node, o.owner, "borrow-conflict", "`"+name+"` is borrowed and cannot be mutated here")
			}
		}
	}
}
