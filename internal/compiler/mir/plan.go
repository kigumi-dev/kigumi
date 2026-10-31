package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
)

// Point addresses the moment right after instruction Inst of block Block;
// Inst == len(insts) is the terminator.
type Point struct{ Block, Inst int }

// use is where a temporary was last read.
type use struct{ block, inst int }

// Plan is the ownership plan every backend shares: where temporaries are
// released, how often each is defined, and which aliases take a reference.
type Plan struct {
	Releases    map[Point][]midir.LocalID
	Splits      map[Edge][]midir.LocalID
	Defs        map[midir.LocalID]int
	Materialize map[midir.LocalID]bool
	LentJoins   map[midir.LocalID]bool
}

// MIR drops only named locals, so a temporary no instruction consumes is
// released after its last use, or by liveness when it crosses blocks.
func (p *Program) planFunc(f *midir.Func) *Plan {
	last := map[midir.LocalID]use{}
	consumed := map[midir.LocalID]bool{}
	crossUse := map[midir.LocalID]bool{}
	reads := map[midir.LocalID]int{}
	useBlocks := map[midir.LocalID]map[int]bool{}
	borrowed := map[midir.LocalID]midir.LocalID{}
	isTemp := func(l midir.LocalID) bool { return f.Locals[l].Ent == 0 }
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
			if (in.Op == midir.OpBorrow || in.Op == midir.OpLendBox || in.Op == midir.OpNewCell && in.Lent) && len(in.Args) == 1 && isTemp(in.Args[0]) {
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
				if blk.Term.Op == midir.TermReturn {
					consumed[a] = true
				}
			}
		}
	}
	life := p.newBorrowLife(f, isTemp, defs, defBlocks, borrowed)
	life.extend(last, crossUse)
	noRelease := p.unownedTemps(f, isTemp, defs, materialize, joins, func(l midir.LocalID) bool { return crossUse[l] && reads[l] > 1 })
	touch := map[midir.LocalID]map[int]bool{}
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
	plan := map[Point][]midir.LocalID{}
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
	var split map[Edge][]midir.LocalID
	if live != nil {
		split = live.edgePoints()
	}
	for _, ts := range plan {
		slices.Sort(ts)
	}
	late.into(plan)
	return &Plan{Releases: plan, Splits: split, Defs: defs, Materialize: materialize, LentJoins: joins}
}

func (p *Program) ArgConsumed(f *midir.Func, in midir.Inst, i int) bool { return p.consumes(f, in, i) }

// consumes reports whether argument i is stored, moved, boxed or handed to
// an owning callee; reads, arithmetic, display and borrowed receivers do not.
func (p *Program) consumes(f *midir.Func, in midir.Inst, i int) bool {
	if p.inPlace(in, i) || p.lentParam(in, i) || p.lentByType(f, &in, i) || p.lentStore(f, in, i) {
		return false
	}
	switch in.Op {
	case midir.OpField, midir.OpFieldMove, midir.OpIndex, midir.OpIsVariant, midir.OpIsType, midir.OpPayload, midir.OpUnbox,
		midir.OpCellGet, midir.OpCellTake, midir.OpBinary, midir.OpUnary, midir.OpInterp, midir.OpBuiltin, midir.OpBorrow, midir.OpLendBox, midir.OpFutureNew, midir.OpFutureDone, midir.OpShare, midir.OpClosure, midir.OpBind:
		return false
	}
	return true
}
