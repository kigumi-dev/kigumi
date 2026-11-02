package borrow

import (
	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// Borrow rules per function on the MIR: no escape (return, store, capture); the owner is untouchable under an exclusive
// borrow and cannot be mutated, moved or dropped under a shared one; liveness is by last use.

// ownerRef is one local a borrow may actually alias, with the field chain
// (see projectionPlace) it was taken through.
type ownerRef struct {
	owner midir.LocalID
	mut   bool
	path  ownquery.Path
	exact bool
	// view names the `lendbox` result whose frame slot this borrow points at.
	view bool
	// carried marks a borrow a container holds through a carried value (see carrySite).
	carried bool
	// lifetime is the parameter lifetime a borrow into a parameter's referent may be returned under.
	lifetime sem.EntityID
	// temp marks a borrow of a call's result temporary, which lives only to the statement's end.
	temp bool
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
	p        *ownquery.Input
	state    *Analyzer
	f        *midir.Func
	holders  map[midir.LocalID]*borrowInfo
	liveOut  []map[midir.LocalID]bool
	liveIn   []map[midir.LocalID]bool
	firstDef map[midir.LocalID]int
	// realOut is liveOut without the scope-end release of a reference, which only ends or replaces nothing.
	realOut []map[midir.LocalID]bool
	// definedIn: holders assigned on some path into a block, since an `is` binding is live at the join before it is defined.
	definedIn []map[midir.LocalID]bool
	reported  map[reportKey]int
	quiet     bool
	types     *TypeFacts
	// defs maps a temporary to its defining instruction (nil when defined
	// more than once), built on first use.
	defs map[midir.LocalID]*midir.Inst
	// sole caches soleDefs for soleDef, built on first use; defsAll caches defsOf.
	sole    map[midir.LocalID]*midir.Inst
	defsAll map[midir.LocalID][]*midir.Inst
	// chainOnly caches chainedOnly's result per local.
	chainOnly map[midir.LocalID]bool
	// unowned caches unownedRefs, built on first use.
	unowned []bool
	// sinks are the holders collect re-merges on every round: containers
	// whose element sources are the union of every write.
	sinks       map[midir.LocalID]*borrowInfo
	targets     map[midir.LocalID]bool
	contents    map[midir.LocalID]*borrowInfo
	caps        map[midir.LocalID]*borrowInfo
	grew        bool
	borrowedIn  []map[midir.LocalID]bool
	firstBorrow map[midir.LocalID]int
	curBlock    midir.BlockID
	loopReach   [][]bool
	curIdx      int
	keptTemp    map[midir.LocalID]bool
	keptOwners  map[midir.LocalID]bool
	carry       map[midir.LocalID][]carrySite
	carryIn     map[[2]midir.LocalID][]bool
	ties        tieState
	held        *heldState
	resets      resetState
	writeBacks  map[carrySite]bool
	recvBorrows map[recvBorrowKey]*borrowInfo
	callNode    syntax.NodeID
	thread      *mirflow.FlagThread
	callRefs    map[midir.LocalID]bool
	reborrows   []reborrowSite
	unions      []unionSite
	mixed       map[syntax.NodeID]midir.LocalID
	viewDefs    map[midir.LocalID][]*midir.Inst
	viewSets    map[midir.LocalID]viewSourceSet
	ended       map[midir.LocalID][]bool

	cellState
	fields fieldState
}

func analyzeBorrows(state *Analyzer, f *midir.Func) {
	p := state.Input
	c := &borrowCheck{p: p, f: f, state: state, holders: map[midir.LocalID]*borrowInfo{}, reported: map[reportKey]int{}, keptTemp: map[midir.LocalID]bool{}}
	c.collect()
	c.checkOffloads()
	if len(c.holders) == 0 {
		return
	}
	c.liveness()
	c.definitions()
	c.held = &heldState{borrows: c.heldBorrows(), in: map[heldBorrow][]bool{}}
	c.borrowReach()
	for bid := range f.Blocks {
		c.checkBlock(midir.BlockID(bid))
	}
	c.checkTempLeaves()
}

// checkInst sees the holders live after in; a conflict is a touch of an owner whose borrow outlives in,
// and a holder outside realAfter waits only on its own release, so only what may drop a value conflicts.
func (c *borrowCheck) checkInst(in midir.Inst, liveAfter, realAfter map[midir.LocalID]bool) {
	c.checkCapture(in)
	c.checkTempKept(in)
	c.checkRecvArgs(in)
	c.checkViewReuse(in, liveAfter)
	c.checkViewSpawn(in)
	c.checkSuspend(in, liveAfter)
	c.checkCellRunArgs(in, liveAfter)
	// A receiver cell's write-back releases the place's old value; the
	// exclusive borrow the call took of that place already reported.
	if in.Op == midir.OpDrop && in.Scoped || in.Op == midir.OpNop {
		return
	}
	c.checkHeld(in)
	for holder := range liveAfter {
		b := c.holders[holder]
		if !b.hasOwner || holder == in.Dst || !realAfter[holder] && !c.mayDrop(in) {
			continue
		}
		for _, o := range b.owners {
			if o.view || o.through && in.Op == midir.OpDrop && in.EndScope || !c.ownerMayBeHeld(holder, o.owner) || o.carried && !c.carryReaches(holder, o.owner, c.curIdx) || o.tied && !c.tieReaches(holder, o.owner, c.curIdx) || !c.resetReaches(holder, o.owner, c.curIdx) {
				continue
			}
			touched, path, _ := c.touch(in, o.owner)
			if hit, _ := c.cellRunHit(in, o); !touched && hit {
				touched, path = true, o.path
			}
			if !touched || !ownquery.FieldPathsOverlap(path, o.path) {
				continue
			}
			c.reportTouch(in, holder, b, o)
		}
	}
}
