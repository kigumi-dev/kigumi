package driver

import (
	"slices"
	"sort"
)

// packageSCCs groups nodes into strongly connected components of the import graph, dependencies first, with a stable order inside each component; deps maps a node to the nodes it imports.
func packageSCCs(nodes []string, deps map[string][]string) [][]string {
	sorted := slices.Clone(nodes)
	sort.Strings(sorted)
	inSet := map[string]bool{}
	for _, n := range sorted {
		inSet[n] = true
	}
	index, low := map[string]int{}, map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	var out [][]string
	counter := 0
	var strong func(v string)
	strong = func(v string) {
		counter++
		index[v], low[v] = counter, counter
		stack = append(stack, v)
		onStack[v] = true
		for _, w := range deps[v] {
			if !inSet[w] {
				continue
			}
			if index[w] == 0 {
				strong(w)
				low[v] = min(low[v], low[w])
			} else if onStack[w] {
				low[v] = min(low[v], index[w])
			}
		}
		if low[v] != index[v] {
			return
		}
		var comp []string
		for {
			w := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[w] = false
			comp = append(comp, w)
			if w == v {
				break
			}
		}
		sort.Strings(comp)
		out = append(out, comp)
	}
	for _, v := range sorted {
		if index[v] == 0 {
			strong(v)
		}
	}
	return out
}
