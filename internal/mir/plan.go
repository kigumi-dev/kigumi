package mir

import "slices"

// Point addresses the moment right after instruction Inst of block Block;
// Inst == len(insts) is the terminator.
type Point struct{ Block, Inst int }

// use is where a temporary was last read.
type use struct{ block, inst int }

// Plan is the ownership plan every backend shares: where temporaries are
// released, how often each is defined, and which aliases take a reference.
type Plan struct {
	Releases    map[Point][]LocalID
	Splits      map[Edge][]LocalID
	Defs        map[LocalID]int
	Materialize map[LocalID]bool
	LentJoins   map[LocalID]bool
}

// MIR drops only named locals, so a temporary no instruction consumes is
// released after its last use, or by liveness when it crosses blocks.
func (p *Program) planFunc(f *Func) *Plan {
	last := map[LocalID]use{}
	consumed := map[LocalID]bool{}
	crossUse := map[LocalID]bool{}
	reads := map[LocalID]int{}
	useBlocks := map[LocalID]map[int]bool{}
	borrowed := map[LocalID]LocalID{}
	isTemp := func(l LocalID) bool { return f.Locals[l].Ent == 0 }
	defs, defBlocks, defBlock := tempDefs(f, isTemp)
	materialize := p.planAliases(f, isTemp, defs)
	joins := p.lentJoins(f, defs)
	// Block numbers do not follow execution order, so a temporary used outside
	// its defining block is released by liveness instead of at its last read.
	for b, blk := range f.Blocks {
		for k, in := range blk.Insts {
			for i, a := range in.Args {
				if !isTemp(a) {
					continue
				}
				if defBlock[a] != b {
					crossUse[a] = true
				}
				noteBlock(useBlocks, a, b)
				reads[a]++
				last[a] = use{b, k}
				if p.consumes(f, in, i) {
					consumed[a] = true
				}
			}
			if !definesTemp(in, isTemp) {
				continue
			}
			if _, used := last[in.Dst]; !used {
				last[in.Dst] = use{b, k}
			}
			if (in.Op == OpBorrow || in.Op == OpLendBox || in.Op == OpNewCell && in.Lent) && len(in.Args) == 1 && isTemp(in.Args[0]) {
				borrowed[in.Dst] = in.Args[0]
			}
		}
		for _, a := range blk.Term.Args {
			if isTemp(a) {
				if defBlock[a] != b {
					crossUse[a] = true
				}
				noteBlock(useBlocks, a, b)
				reads[a]++
				last[a] = use{b, len(blk.Insts)}
				if blk.Term.Op == TermReturn {
					consumed[a] = true
				}
			}
		}
	}
	life := p.newBorrowLife(f, isTemp, defs, defBlocks, borrowed)
	life.extend(last, crossUse)
	noRelease := p.unownedTemps(f, isTemp, defs, materialize, joins, func(l LocalID) bool { return crossUse[l] && reads[l] > 1 })
	touch := map[LocalID]map[int]bool{}
	for t, bs := range defBlocks {
		for b := range bs {
			noteBlock(touch, t, b)
		}
	}
	for t, bs := range useBlocks {
		for b := range bs {
			for _, r := range life.keeps(t) {
				noteBlock(touch, r, b)
			}
		}
	}
	plan := map[Point][]LocalID{}
	late := lateReleases{}
	var live *liveCtx
	for t, u := range last {
		if noRelease[t] {
			continue
		}
		if consumed[t] {
			if crossUse[t] || defs[t] != 1 {
				if live == nil {
					live = newLiveCtx(f, isTemp, life, plan)
				}
				live.placeAbandoned(t, touch[t])
			}
			continue
		}
		if !crossUse[t] && defs[t] == 1 && defBlock[t] == u.block {
			if p.releasedLate(f, t, u) {
				late.add(f, t, u)
				continue
			}
			plan[Point{u.block, u.inst}] = append(plan[Point{u.block, u.inst}], t)
			continue
		}
		if live == nil {
			live = newLiveCtx(f, isTemp, life, plan)
		}
		live.place(t, touch[t])
	}
	var split map[Edge][]LocalID
	if live != nil {
		split = live.edgePoints()
	}
	for _, ts := range plan {
		slices.Sort(ts)
	}
	late.into(plan)
	return &Plan{Releases: plan, Splits: split, Defs: defs, Materialize: materialize, LentJoins: joins}
}

func (p *Program) ArgConsumed(f *Func, in Inst, i int) bool { return p.consumes(f, in, i) }

// consumes reports whether argument i is stored, moved, boxed or handed to
// an owning callee; reads, arithmetic, display and borrowed receivers do not.
func (p *Program) consumes(f *Func, in Inst, i int) bool {
	if p.inPlace(in, i) || p.lentParam(in, i) || p.lentByType(f, &in, i) || p.lentStore(f, in, i) {
		return false
	}
	switch in.Op {
	case OpField, OpFieldMove, OpIndex, OpIsVariant, OpIsType, OpPayload, OpUnbox,
		OpCellGet, OpCellTake, OpBinary, OpUnary, OpInterp, OpBuiltin, OpBorrow, OpLendBox, OpFutureNew, OpFutureDone, OpShare, OpClosure, OpBind:
		return false
	}
	return true
}
