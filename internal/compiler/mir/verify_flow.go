package mir

import (
	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
)

func solveForward(f *midir.Func, visit func(bid midir.BlockID, wake func(midir.BlockID), final bool) bool) (acyclic bool) {
	order := mirflow.ReversePostorder(f)
	rank := make([]int, len(f.Blocks))
	for i, bid := range order {
		rank[bid] = i
	}
	final := true
	for _, bid := range order {
		for _, succ := range f.Blocks[bid].Term.Targets {
			if rank[succ] <= rank[bid] {
				final = false
			}
		}
	}
	pending := make([]bool, len(f.Blocks))
	pending[0] = true
	for again := true; again; {
		again = false
		for _, bid := range order {
			if !pending[bid] {
				continue
			}
			pending[bid] = false
			if !visit(bid, func(succ midir.BlockID) {
				pending[succ] = true
				again = true
			}, final) {
				return final
			}
		}
	}
	return final
}

func cyclicBlocks(f *midir.Func) []bool {
	n := len(f.Blocks)
	index, low := make([]int, n), make([]int, n)
	onStack := make([]bool, n)
	cyclic := make([]bool, n)
	var stack []midir.BlockID
	next := 1
	var visit func(midir.BlockID)
	visit = func(b midir.BlockID) {
		index[b], low[b] = next, next
		next++
		stack = append(stack, b)
		onStack[b] = true
		for _, succ := range f.Blocks[b].Term.Targets {
			switch {
			case index[succ] == 0:
				visit(succ)
				low[b] = min(low[b], low[succ])
			case onStack[succ]:
				low[b] = min(low[b], index[succ])
			}
			if succ == b {
				cyclic[b] = true
			}
		}
		if low[b] != index[b] {
			return
		}
		var members []midir.BlockID
		for {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[top] = false
			members = append(members, top)
			if top == b {
				break
			}
		}
		if len(members) > 1 {
			for _, m := range members {
				cyclic[m] = true
			}
		}
	}
	visit(0)
	return cyclic
}
