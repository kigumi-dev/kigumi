package unwind

import (
	"slices"

	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// Plan lists, per instruction, the owned locals a panic there must release: those used or dropped after it, bar its own result.
type Plan struct {
	// Sets[0] is empty; each set runs from the last declared local to the first.
	Sets [][]midir.LocalID
	// At is -1 where the instruction cannot panic.
	At [][]int32
}

func (u *Plan) SetAt(b midir.BlockID, pc int) []midir.LocalID {
	if int(b) >= len(u.At) || pc >= len(u.At[b]) || u.At[b][pc] < 0 {
		return nil
	}
	return u.Sets[u.At[b][pc]]
}

// Nothing here allocates, charges, runs a drop hook or checks a bound.
func cannotPanic(op midir.Opcode) bool {
	switch op {
	case midir.OpConst, midir.OpUnit, midir.OpAlias, midir.OpMove, midir.OpCopy, midir.OpNop, midir.OpFnItem, midir.OpShare,
		midir.OpField, midir.OpFieldMove, midir.OpPayload, midir.OpTag, midir.OpIsVariant, midir.OpIsType, midir.OpUnbox, midir.OpBorrow, midir.OpCellGet, midir.OpCellTake:
		return true
	}
	return false
}

// candidates ignores frame slots, since the plan decides which temporaries get one.
func Compute(f *midir.Func, r *sem.Result, candidates bool, lent map[midir.LocalID]bool, held func(in *midir.Inst, i int) bool) *Plan {
	plan := &Plan{Sets: [][]midir.LocalID{nil}, At: make([][]int32, len(f.Blocks))}
	for i := range f.Blocks {
		plan.At[i] = make([]int32, len(f.Blocks[i].Insts))
		for j := range plan.At[i] {
			plan.At[i][j] = -1
		}
	}
	owned := unwindOwned(f, r, candidates)
	for l := range lent {
		owned[l] = false
	}
	words := (len(f.Locals) + 63) / 64
	liveIn := make([]mirflow.Bitset, len(f.Blocks))
	for i := range liveIn {
		liveIn[i] = make(mirflow.Bitset, words)
	}
	walk := func(bid int) mirflow.Bitset {
		blk := &f.Blocks[bid]
		live := make(mirflow.Bitset, words)
		for _, t := range blk.Term.Targets {
			for w := range live {
				live[w] |= liveIn[t][w]
			}
		}
		for _, a := range blk.Term.Args {
			if owned[a] {
				live.Set(a)
			}
		}
		for i := len(blk.Insts) - 1; i >= 0; i-- {
			in := &blk.Insts[i]
			if in.Op.Yields() {
				live.Clear(in.Dst)
			}
			for _, a := range in.Args {
				if owned[a] {
					live.Set(a)
				}
			}
		}
		return live
	}
	for changed := true; changed; {
		changed = false
		for bid := len(f.Blocks) - 1; bid >= 0; bid-- {
			if live := walk(bid); !slices.Equal(live, liveIn[bid]) {
				liveIn[bid] = live
				changed = true
			}
		}
	}
	fill := &unwindFill{f: f, plan: plan, owned: owned, parent: AliasParents(f), held: held, words: words, interned: map[uint64][]int32{}}
	fill.aliased = make([]bool, len(f.Locals))
	for l := range fill.parent {
		fill.aliased[l] = true
	}
	for bid := range f.Blocks {
		fill.block(bid, liveIn)
	}
	return plan
}

// An alias holds the reference of the local it copies: an unowned source means nothing to release, a live one releases it once.
func sharesOwner(id midir.LocalID, parent map[midir.LocalID]midir.LocalID, owned map[midir.LocalID]bool, live mirflow.Bitset, in *midir.Inst) bool {
	src := id
	for range len(parent) + 1 {
		next, ok := parent[src]
		if !ok {
			return false
		}
		src = next
		if !owned[src] || live.Has(src) && !(in.Op.Yields() && in.Dst == src) {
			return true
		}
	}
	return false
}
