package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
)

// A drop, a move or a new definition ends a local's value; a release reads nothing.
type liveness struct {
	blocks []midir.Block
	n      int
	out    [][]uint64
}

func newLiveness(blocks []midir.Block, n int) *liveness {
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
			lv.blockIn(midir.BlockID(b), live)
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

func (lv *liveness) blockIn(b midir.BlockID, live []uint64) {
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

func reads(op midir.Opcode) bool { return op != midir.OpDrop && op != midir.OpRelease }

func ends(in midir.Inst) []midir.LocalID {
	var out []midir.LocalID
	if in.Op.Yields() {
		out = append(out, in.Dst)
	}
	switch in.Op {
	case midir.OpDrop, midir.OpRelease, midir.OpMove:
		out = append(out, in.Args...)
	}
	return out
}

func (lv *liveness) after(l midir.LocalID, b, k int) bool {
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
