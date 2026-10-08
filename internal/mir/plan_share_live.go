package mir

import "slices"

// A drop, a move or a new definition ends a local's value; a release reads nothing.
type liveness struct {
	blocks []Block
	n      int
	out    [][]uint64
}

func newLiveness(blocks []Block, n int) *liveness {
	words := (n + 63) / 64
	lv := &liveness{blocks: blocks, n: n, out: make([][]uint64, len(blocks))}
	in := make([][]uint64, len(blocks))
	for b := range blocks {
		lv.out[b], in[b] = make([]uint64, words), make([]uint64, words)
	}
	for changed := true; changed; {
		changed = false
		for b := len(blocks) - 1; b >= 0; b-- {
			for _, s := range blocks[b].Term.Targets {
				for i, x := range in[s] {
					lv.out[b][i] |= x
				}
			}
			live := append([]uint64(nil), lv.out[b]...)
			lv.blockIn(BlockID(b), live)
			for i := range live {
				if live[i] != in[b][i] {
					in[b], changed = live, true
					break
				}
			}
		}
	}
	return lv
}

func (lv *liveness) blockIn(b BlockID, live []uint64) {
	blk := &lv.blocks[b]
	for _, a := range blk.Term.Args {
		live[a/64] |= 1 << (a % 64)
	}
	for k := len(blk.Insts) - 1; k >= 0; k-- {
		in := &blk.Insts[k]
		for _, l := range ends(*in) {
			live[l/64] &^= 1 << (l % 64)
		}
		if reads(in.Op) {
			for _, a := range in.Args {
				live[a/64] |= 1 << (a % 64)
			}
		}
	}
}

func reads(op Opcode) bool { return op != OpDrop && op != OpRelease }

func ends(in Inst) []LocalID {
	var out []LocalID
	if in.Op.Yields() {
		out = append(out, in.Dst)
	}
	switch in.Op {
	case OpDrop, OpRelease, OpMove:
		out = append(out, in.Args...)
	}
	return out
}

func (lv *liveness) after(l LocalID, b, k int) bool {
	if int(l) >= lv.n {
		return false
	}
	blk := &lv.blocks[b]
	for j := k + 1; j < len(blk.Insts); j++ {
		in := blk.Insts[j]
		if reads(in.Op) && slices.Contains(in.Args, l) {
			return true
		}
		if slices.Contains(ends(in), l) {
			return false
		}
	}
	if slices.Contains(blk.Term.Args, l) {
		return true
	}
	return lv.out[b][l/64]&(1<<(l%64)) != 0
}

// blockReach[b][b] holds only through a loop.
func blockReach(blocks []Block) [][]bool {
	out := make([][]bool, len(blocks))
	for b := range blocks {
		seen := make([]bool, len(blocks))
		work := append([]BlockID{}, blocks[b].Term.Targets...)
		for len(work) > 0 {
			c := work[len(work)-1]
			work = work[:len(work)-1]
			if seen[c] {
				continue
			}
			seen[c] = true
			work = append(work, blocks[c].Term.Targets...)
		}
		out[b] = seen
	}
	return out
}
