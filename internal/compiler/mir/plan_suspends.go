package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
)

// liveAtSuspends is a backward liveness over every local; entry k holds
// what is live right after the suspension point numbered k.
func liveAtSuspends(f *midir.Func) []bitset {
	n := len(f.Blocks)
	words := (len(f.Locals) + 63) / 64
	liveIn := make([]bitset, n)
	for i := range liveIn {
		liveIn[i] = make(bitset, words)
	}
	suspended := make([]bitset, f.Suspends+1)
	for i := range suspended {
		suspended[i] = make(bitset, words)
	}
	live := make(bitset, words)
	sweep := backwardOrder(f)
	for changed := true; changed; {
		changed = false
		for _, bid := range sweep {
			blk := &f.Blocks[bid]
			clear(live)
			for _, t := range blk.Term.Targets {
				for w, x := range liveIn[t] {
					live[w] |= x
				}
			}
			for _, a := range blk.Term.Args {
				live.set(a)
			}
			for i := len(blk.Insts) - 1; i >= 0; i-- {
				in := &blk.Insts[i]
				if in.Op == midir.OpAwait && in.Index < len(suspended) {
					for w, x := range live {
						suspended[in.Index][w] |= x
					}
				}
				if in.Op.Yields() {
					live.clear(in.Dst)
				}
				for _, a := range in.Args {
					live.set(a)
				}
			}
			if !slices.Equal(live, liveIn[bid]) {
				copy(liveIn[bid], live)
				changed = true
			}
		}
	}
	return suspended
}

// backwardOrder lists unreachable blocks, then reachable ones in postorder, which settles a backward flow in few sweeps.
func backwardOrder(f *midir.Func) []midir.BlockID {
	rpo := reversePostorder(f)
	reached := make([]bool, len(f.Blocks))
	for _, bid := range rpo {
		reached[bid] = true
	}
	var order []midir.BlockID
	for bid := range f.Blocks {
		if !reached[bid] {
			order = append(order, midir.BlockID(bid))
		}
	}
	for i := len(rpo) - 1; i >= 0; i-- {
		order = append(order, rpo[i])
	}
	return order
}
