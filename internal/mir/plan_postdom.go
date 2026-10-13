package mir

import "math/bits"

// immediatePostDom is the nearest block every path from b to a return passes.
func (c *joinCtx) immediatePostDom(b int) (int, bool) {
	if c.pdom == nil {
		c.pdom = c.postDominators()
	}
	want := count(c.pdom[b]) - 1
	for j := range c.f.Blocks {
		if j != b && has(c.pdom[b], j) && count(c.pdom[j]) == want {
			return j, true
		}
	}
	return 0, false
}

func (c *joinCtx) postDominators() [][]uint64 {
	n := len(c.f.Blocks)
	words := (n + 63) / 64
	sets := make([][]uint64, n)
	for b := range sets {
		sets[b] = make([]uint64, words)
		if len(c.f.Blocks[b].Term.Targets) == 0 {
			sets[b][b/64] |= 1 << (b % 64)
			continue
		}
		for w := range sets[b] {
			sets[b][w] = ^uint64(0)
		}
	}
	for changed := true; changed; {
		changed = false
		for b := n - 1; b >= 0; b-- {
			targets := c.f.Blocks[b].Term.Targets
			if len(targets) == 0 {
				continue
			}
			next := append([]uint64(nil), sets[targets[0]]...)
			for _, t := range targets[1:] {
				for w := range next {
					next[w] &= sets[t][w]
				}
			}
			next[b/64] |= 1 << (b % 64)
			for w := range next {
				if next[w] != sets[b][w] {
					sets[b], changed = next, true
					break
				}
			}
		}
	}
	return sets
}

func has(set []uint64, b int) bool { return set[b/64]>>(b%64)&1 == 1 }

func count(set []uint64) int {
	n := 0
	for _, w := range set {
		n += bits.OnesCount64(w)
	}
	return n
}
