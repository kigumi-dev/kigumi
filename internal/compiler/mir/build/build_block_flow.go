package build

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
)

// blockFlow holds a function's dominator and post-dominator sets; dom[b][a] says a dominates b.
// A block ending in `unreachable` constrains no post-dominator: a panic leaves through unwinding.
type blockFlow struct {
	f     *midir.Func
	preds [][]int
	dom   [][]bool
	pdom  [][]bool
}

func newBlockFlow(f *midir.Func) *blockFlow {
	n := len(f.Blocks)
	preds := make([][]int, n)
	for b, blk := range f.Blocks {
		for _, t := range blk.Term.Targets {
			preds[t] = append(preds[t], b)
		}
	}
	succs := func(b int) []int {
		var out []int
		for _, t := range f.Blocks[b].Term.Targets {
			out = append(out, int(t))
		}
		return out
	}
	fl := &blockFlow{f: f, preds: preds}
	fl.dom = solveDominators(n, func(b int) bool { return b == 0 }, func(b int) []int { return preds[b] })
	fl.pdom = solveDominators(n, func(b int) bool { return f.Blocks[b].Term.Op == midir.TermReturn }, succs)
	return fl
}

// solveDominators intersects, from each root, the sets along next; a block reaching no root keeps the full set.
func solveDominators(n int, root func(int) bool, next func(int) []int) [][]bool {
	sets := make([][]bool, n)
	for b := range sets {
		sets[b] = make([]bool, n)
		for a := range sets[b] {
			sets[b][a] = !root(b) || a == b
		}
	}
	for changed := true; changed; {
		changed = false
		for b := range n {
			if root(b) || len(next(b)) == 0 {
				continue
			}
			for a := range n {
				keep := a == b || !slices.ContainsFunc(next(b), func(x int) bool { return !sets[x][a] })
				if sets[b][a] && !keep {
					sets[b][a], changed = false, true
				}
			}
		}
	}
	return sets
}

// idom is b's nearest strict dominator, -1 for none.
func (fl *blockFlow) idom(b int) int {
	best, most := -1, -1
	for a, ok := range fl.dom[b] {
		if !ok || a == b {
			continue
		}
		if c := count(fl.dom[a]); c > most {
			best, most = a, c
		}
	}
	return best
}

func count(s []bool) int {
	n := 0
	for _, v := range s {
		if v {
			n++
		}
	}
	return n
}

func (fl *blockFlow) postDominates(a, b int) bool { return fl.pdom[b][a] }

func (fl *blockFlow) liveOut(b int, live []bool) bool {
	return slices.ContainsFunc(fl.f.Blocks[b].Term.Targets, func(t midir.BlockID) bool { return live[t] })
}
