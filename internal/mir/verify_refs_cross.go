package mir

func (rf *refFlow) crossedFromTemp(bid BlockID, i int, l LocalID) bool {
	f := rf.f
	if !f.Locals[l].Cross {
		return false
	}
	blk := f.Blocks[bid]
	j := adjacentDef(f, blk, i, l)
	if j < 0 || len(blk.Insts[j].Args) != 1 || (blk.Insts[j].Op != OpBoxDirect && blk.Insts[j].Op != OpUnboxDirect) {
		return false
	}
	return f.Locals[blk.Insts[j].Args[0]].Ent == 0
}
