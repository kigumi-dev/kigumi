package llgen

import "kigumi/internal/compiler/mir"

// copyStoreInsts merges each `t = copy s; setindex b, i, t` (or `setfield b.f, t`) into one in-place store: no box is built for t.
func copyStoreInsts(blk *mir.Block, bid int, stores []mir.CopyStore) []mir.Inst {
	var mine []mir.CopyStore
	for _, cs := range stores {
		if int(cs.Block) == bid {
			mine = append(mine, cs)
		}
	}
	if len(mine) == 0 {
		return blk.Insts
	}
	out := append([]mir.Inst(nil), blk.Insts...)
	for _, cs := range mine {
		set := out[cs.Set]
		src := out[cs.Share].Args[0]
		set.Copy, set.Args = true, append(append([]mir.LocalID(nil), set.Args[:len(set.Args)-1]...), src)
		out[cs.Set], out[cs.Share] = set, mir.Inst{Op: mir.OpNop}
	}
	return out
}
