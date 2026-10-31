package mir

import (
	"maps"
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
)

// pruneJoins keeps a join's operand only when every source it carries is made before each definition of the join:
// a source made on one arm has no definition, and so no release, on the other arm's path.
func (b *borrowLife) pruneJoins(f *midir.Func, defs map[midir.LocalID]int, defBlocks map[midir.LocalID]map[int]bool) {
	var g *flowGraph
	keep := map[midir.LocalID][]midir.LocalID{}
	for _, j := range slices.Sorted(maps.Keys(b.carried)) {
		if defs[j] < 2 {
			continue
		}
		if g == nil {
			g = newFlowGraph(f)
		}
		keep[j] = slices.DeleteFunc(slices.Clone(b.carried[j]), func(c midir.LocalID) bool {
			return slices.ContainsFunc(b.through(c), func(s midir.LocalID) bool {
				return defs[s] != 1 || !madeBefore(g, defBlocks[s], defBlocks[j])
			})
		})
	}
	for j, cs := range keep {
		if len(cs) == 0 {
			delete(b.carried, j)
		} else {
			b.carried[j] = cs
		}
	}
	clear(b.memo)
}

// madeBefore reports whether the one block in made lies on every path from the entry to each block in uses.
func madeBefore(g *flowGraph, made map[int]bool, uses map[int]bool) bool {
	var d int
	for b := range made {
		d = b
	}
	if d == 0 {
		return true
	}
	seen := map[int]bool{0: true, d: true}
	work := []int{0}
	for len(work) > 0 {
		x := work[len(work)-1]
		work = work[:len(work)-1]
		if uses[x] {
			return false
		}
		for _, y := range g.succ(x) {
			if !seen[int(y)] {
				seen[int(y)] = true
				work = append(work, int(y))
			}
		}
	}
	return true
}
