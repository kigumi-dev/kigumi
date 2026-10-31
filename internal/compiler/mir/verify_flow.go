package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
)

func reversePostorder(f *midir.Func) []midir.BlockID {
	seen := make([]bool, len(f.Blocks))
	var post []midir.BlockID
	type frame struct {
		bid  midir.BlockID
		next int
	}
	stack := []frame{{0, 0}}
	seen[0] = true
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		targets := f.Blocks[top.bid].Term.Targets
		if top.next < len(targets) {
			succ := targets[top.next]
			top.next++
			if !seen[succ] {
				seen[succ] = true
				stack = append(stack, frame{succ, 0})
			}
			continue
		}
		post = append(post, top.bid)
		stack = stack[:len(stack)-1]
	}
	for i, j := 0, len(post)-1; i < j; i, j = i+1, j-1 {
		post[i], post[j] = post[j], post[i]
	}
	return post
}

func solveForward(f *midir.Func, visit func(bid midir.BlockID, wake func(midir.BlockID), final bool) bool) (acyclic bool) {
	order := reversePostorder(f)
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
