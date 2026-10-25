package mir

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// Borrow rules per function on the MIR: no escape (return, store, capture); the owner is untouchable under an exclusive
// borrow and cannot be mutated, moved or dropped under a shared one; liveness is by last use.

// ownerRef is one local a borrow may actually alias, with the field chain
// (see projectionPlace) it was taken through.
type ownerRef struct {
	owner LocalID
	mut   bool
	path  []int
	exact bool
	// view names the `lendbox` result whose frame slot this borrow points at.
	view bool
	// carried marks a borrow a container holds through a carried value (see carrySite).
	carried bool
	// lifetime is the parameter lifetime a borrow into a parameter's referent may be returned under.
	lifetime sem.EntityID
	// temp marks a borrow of a call's result temporary, which lives only to the statement's end.
	temp bool
	// cell marks a borrow of a closure-shared local's cell; sem judges its conflicts with the closures.
	cell bool
	// caller marks a borrow of a writable caller place, kept only to conflict with this body's writes.
	caller bool
	// node is where a store put this borrow into its holder, 0 for the holder's own node.
	node syntax.NodeID
	// through marks a call result's borrow through a reference local; the result holds its own count, so the local may end first.
	through bool
	tied    bool
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
	p        *Program
	f        *Func
	holders  map[LocalID]*borrowInfo
	liveOut  []map[LocalID]bool
	liveIn   []map[LocalID]bool
	firstDef map[LocalID]int
	// realOut is liveOut without the scope-end release of a reference, which only ends or replaces nothing.
	realOut []map[LocalID]bool
	// definedIn: holders assigned on some path into a block, since an `is` binding is live at the join before it is defined.
	definedIn []map[LocalID]bool
	reported  map[reportKey]int
	quiet     bool
	typeMemo  borrowTypeMemo
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
	loopReach   [][]bool
	curIdx      int
	keptTemp    map[LocalID]bool
	keptOwners  map[LocalID]bool
	carry       map[LocalID][]carrySite
	carryIn     map[[2]LocalID][]bool
	ties        tieState
	held        *heldState
	resets      resetState
	recvBorrows map[recvBorrowKey]*borrowInfo
	callNode    syntax.NodeID
	thread      *flagThread
	callRefs    map[LocalID]bool
	reborrows   []reborrowSite
	unions      []unionSite
	mixed       map[syntax.NodeID]LocalID
}

func analyzeBorrows(p *Program, f *Func) {
	c := &borrowCheck{p: p, f: f, holders: map[LocalID]*borrowInfo{}, reported: map[reportKey]int{}, keptTemp: map[LocalID]bool{}}
	c.collect()
	if len(c.holders) == 0 {
		return
	}
	c.liveness()
	c.definitions()
	c.held = &heldState{borrows: c.heldBorrows(), in: map[heldBorrow][]bool{}}
	c.borrowReach()
	for bid := range f.Blocks {
		c.checkBlock(BlockID(bid))
	}
	c.checkTempLeaves()
}

// checkBlock walks a block backward, tracking the holders live after each
// instruction, and reports the conflicts and escapes.
func (c *borrowCheck) checkBlock(bid BlockID) {
	blk := &c.f.Blocks[bid]
	live, real := copySet(c.liveOut[bid]), copySet(c.realOut[bid])
	for _, a := range blk.Term.Args {
		if _, ok := c.holders[a]; ok {
			real[a] = true
		}
		if b, ok := c.holders[a]; ok {
			if c.callerCells(b) || c.callerKept(b) {
				live[a] = true
				continue
			}
			// A holder with no owner is a borrow inherited from a
			// parameter, not one taken of a local: returning it is the
			// intended lifetime escape, and the caller's
			// own call site becomes the new holder (lifetimeSource).
			if !b.hasOwner {
				c.checkLifetimeEscape(a, b)
				live[a] = true
				continue
			}
			if lt := referentLifetime(b); lt != 0 {
				c.checkLifetimeEscape(a, &borrowInfo{lifetime: lt, node: b.node})
				live[a] = true
				continue
			}
			c.report(b.node, a, "borrow-escape", returnEscapeMessage(b))
			live[a] = true
		}
	}
	c.enterDefs(bid)
	c.enterBlock(bid)
	for i := len(blk.Insts) - 1; i >= 0; i-- {
		in := blk.Insts[i]
		c.curIdx = i
		c.checkInst(in, c.definedLive(live, bid, i), real)
		c.stepBack(in, live, c.plainRefRelease)
		c.stepBack(in, real, c.refRelease)
	}
}

// checkInst sees the holders live after in; a conflict is a touch of an owner whose borrow outlives in,
// and a holder outside realAfter waits only on its own release, so only what may drop a value conflicts.
func (c *borrowCheck) checkInst(in Inst, liveAfter, realAfter map[LocalID]bool) {
	c.checkCapture(in)
	c.checkTempKept(in)
	c.checkRecvArgs(in)
	c.checkViewReuse(in, liveAfter)
	c.checkViewSpawn(in)
	c.checkSuspend(in, liveAfter)
	// A receiver cell's write-back releases the place's old value; the
	// exclusive borrow the call took of that place already reported.
	if in.Op == OpDrop && in.Scoped || in.Op == OpNop {
		return
	}
	c.checkHeld(in)
	for holder := range liveAfter {
		b := c.holders[holder]
		if !b.hasOwner || holder == in.Dst || !realAfter[holder] && !c.mayDrop(in) {
			continue
		}
		for _, o := range b.owners {
			if o.view || o.cell || o.through && in.Op == OpDrop && in.EndScope || !c.mayBeBorrowed(o.owner, c.curIdx) || o.carried && !c.carryReaches(holder, o.owner, c.curIdx) || o.tied && !c.tieReaches(holder, o.owner, c.curIdx) || !c.resetReaches(holder, o.owner, c.curIdx) {
				continue
			}
			touched, path, _ := c.touch(in, o.owner)
			if !touched || !fieldPathsOverlap(path, o.path) {
				continue
			}
			c.reportTouch(in, holder, b, o)
		}
	}
}
