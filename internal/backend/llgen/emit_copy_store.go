package llgen

import (
	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
)

// copyStoreInsts merges each `t = copy s; setindex b, i, t` (or `setfield b.f, t`) into one in-place store: no box is built for t.
func copyStoreInsts(blk *midir.Block, bid int, stores []mir.CopyStore) []midir.Inst {
	var mine []mir.CopyStore
	for _, cs := range stores {
		if int(cs.Block) == bid {
			mine = append(mine, cs)
		}
	}
	if len(mine) == 0 {
		return blk.Insts
	}
	out := append([]midir.Inst(nil), blk.Insts...)
	for _, cs := range mine {
		set := out[cs.Set]
		src := out[cs.Share].Args[0]
		set.Copy, set.Args = true, append(append([]midir.LocalID(nil), set.Args[:len(set.Args)-1]...), src)
		out[cs.Set], out[cs.Share] = set, midir.Inst{Op: midir.OpNop}
	}
	return out
}
