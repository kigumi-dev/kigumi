package verify

import midir "kigumi/internal/compiler/mir/ir"

func (rf *refFlow) crossedFromTemp(p *verifier, bid midir.BlockID, i int, l midir.LocalID) bool {
	f := rf.f
	if !f.Locals[l].Cross {
		return false
	}
	blk := f.Blocks[bid]
	j := adjacentDef(f, blk, i, l)
	if j < 0 || len(blk.Insts[j].Args) != 1 || (blk.Insts[j].Op != midir.OpBoxDirect && blk.Insts[j].Op != midir.OpUnboxDirect) {
		return false
	}
	return p.CrossHandsOver(f, &blk.Insts[j])
}
