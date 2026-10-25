package mir

import (
	"slices"

	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// cellState is what the checks of closure-shared cells compute once per body.
type cellState struct {
	closures        *valueFlow
	borrowFlow      *valueFlow
	aliasMemo       map[LocalID]bool
	typeRunMemo     map[typeRunKey]bool
	spawnMemo       map[sem.EntityID]bool
	runMemo         map[sem.EntityID]bool
	closureTypeMemo map[sem.TypeID][]sem.EntityID
	closureUseMemo  map[closureUseKey][]cellUse
	storeMemo       map[paramKey]*siteEntry
	siteSCC         sccStack[paramKey]
	siteLow         int
	siteRuns        int
	contentsMemo    map[LocalID]contentsResult
	plainTypes      map[sem.TypeID]bool
	offloadNode     map[sem.EntityID]syntax.NodeID
}

// dropFact marks, among the closure facts, a value that may hold a resource whose user `drop` runs when it is dropped.
const dropFact uint32 = 0

// heldClosures are the closure literals a may hold just before the instruction being checked, and dropFact.
func (c *borrowCheck) heldClosures(a LocalID) map[uint32]bool {
	if c.closures == nil {
		c.closures = c.solveFlow(func(in Inst) []uint32 {
			var out []uint32
			if in.Op == OpClosure && c.p.R.Entity(in.Ent).Kind == sem.EntClosure {
				out = append(out, uint32(in.Ent))
			}
			if in.Op.Yields() && c.typeRuns(c.f.Locals[in.Dst].Type, runsDrop) {
				out = append(out, dropFact)
			}
			return out
		}, c.spawnStep)
	}
	return c.flowAt(c.closures, a)
}

// dropRuns reports whether dropping a may run a user `drop`: by its type, or by a value it was built from (a Future
// holding a call's argument, a closure that moved one in).
func (c *borrowCheck) dropRuns(a LocalID) bool {
	return c.typeRuns(c.f.Locals[a].Type, runsDrop) || c.heldClosures(a)[dropFact]
}

// ownerMayBeHeld reports whether holder may hold a borrow of owner here: a widening container only once a borrow
// of owner has flowed into it, so an earlier borrow that ended elsewhere does not count; any other holder as before.
func (c *borrowCheck) ownerMayBeHeld(holder, owner LocalID) bool {
	if _, sink := c.sinks[holder]; !sink || !c.mayBeBorrowed(owner, c.curIdx) {
		return c.mayBeBorrowed(owner, c.curIdx)
	}
	if c.borrowFlow == nil {
		c.borrowFlow = c.solveFlow(c.flowBorrowRoots, nil)
	}
	// An owner the flow never carries into holder, or a holder something else can write through, may take the
	// owner on a route the flow does not model: keep the reach rule.
	f := uint32(owner)
	return !c.borrowFlow.ever[holder][f] || c.mutAliased(holder) || c.flowHas(c.borrowFlow, holder, f)
}

// mutAliased reports a local some `&mut` of it outlives the call it is lent to: kept in a record, a binding or a future.
func (c *borrowCheck) mutAliased(l LocalID) bool {
	if v, ok := c.aliasMemo[l]; ok {
		return v
	}
	if c.aliasMemo == nil {
		c.aliasMemo = map[LocalID]bool{}
	}
	aliased := false
	for _, blk := range c.f.Blocks {
		for _, in := range blk.Insts {
			if in.Op != OpBorrow || in.Index != 1 {
				continue
			}
			if root, _, _, ok := c.projectionPlace(in.Args[0]); ok && root == l && !c.lentToCall(in.Dst, 0) {
				aliased = true
			}
		}
	}
	c.aliasMemo[l] = aliased
	return aliased
}

// lentToCall reports a reference whose every use is an argument of a call, directly or through a write-back cell.
func (c *borrowCheck) lentToCall(l LocalID, depth int) bool {
	if depth > 8 || c.f.Locals[l].Ent != 0 {
		return false
	}
	for _, blk := range c.f.Blocks {
		if slices.Contains(blk.Term.Args, l) {
			return false
		}
		for _, in := range blk.Insts {
			if !slices.Contains(in.Args, l) {
				continue
			}
			switch in.Op {
			case OpCall, OpCallValue, OpBuiltin, OpCellTake, OpCellGet, OpDrop, OpRelease:
			case OpShare, OpNewCell, OpAlias, OpCopy:
				if !c.lentToCall(in.Dst, depth+1) {
					return false
				}
			default:
				return false
			}
		}
	}
	return true
}

// flowBorrowRoots is borrowRoots as a flow seed.
func (c *borrowCheck) flowBorrowRoots(in Inst) []uint32 {
	var out []uint32
	for _, root := range c.borrowRoots(in) {
		out = append(out, uint32(root))
	}
	return out
}
