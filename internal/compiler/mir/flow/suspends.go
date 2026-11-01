package flow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
)

// LiveAtSuspends is a backward liveness over every local; entry k holds
// what is live right after the suspension point numbered k.
func LiveAtSuspends(f *midir.Func) []Bitset {
	n := len(f.Blocks)
	words := (len(f.Locals) + 63) / 64
	liveIn := make([]Bitset, n)
	for i := range liveIn {
		liveIn[i] = make(Bitset, words)
	}
	suspended := make([]Bitset, f.Suspends+1)
	for i := range suspended {
		suspended[i] = make(Bitset, words)
	}
	live := make(Bitset, words)
	sweep := BackwardOrder(f)
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
				live.Set(a)
			}
			for i := len(blk.Insts) - 1; i >= 0; i-- {
				in := &blk.Insts[i]
				if in.Op == midir.OpAwait && in.Index < len(suspended) {
					for w, x := range live {
						suspended[in.Index][w] |= x
					}
				}
				if in.Op.Yields() {
					live.Clear(in.Dst)
				}
				for _, a := range in.Args {
					live.Set(a)
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
