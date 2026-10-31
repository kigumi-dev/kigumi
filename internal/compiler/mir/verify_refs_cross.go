package mir

func (rf *refFlow) crossedFromTemp(p *Program, bid BlockID, i int, l LocalID) bool {
	f := rf.f
	if !f.Locals[l].Cross {
		return false
	}
	blk := f.Blocks[bid]
	j := adjacentDef(f, blk, i, l)
	if j < 0 || len(blk.Insts[j].Args) != 1 || (blk.Insts[j].Op != OpBoxDirect && blk.Insts[j].Op != OpUnboxDirect) {
		return false
	}
	return p.CrossHandsOver(f, &blk.Insts[j])
}

// CrossHandsOver reports whether consuming the crossing gives its source's count back:
// a named local keeps its value, and a Lent crossing reads a reference that holds no count.
func (p *Program) CrossHandsOver(f *Func, cross *Inst) bool {
	return f.Locals[cross.Args[0]].Ent == 0 && !cross.Lent
}
