package mir

import "kigumi/internal/sem"

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
	return p.CrossHandsOver(f, blk.Insts[j].Args[0])
}

// CrossHandsOver reports whether consuming a crossing out of src gives src's count back:
// a named local keeps its value, and a reference holds no count of its referent.
func (p *Program) CrossHandsOver(f *Func, src LocalID) bool {
	return f.Locals[src].Ent == 0 && p.R.Types.Kind(f.Locals[src].Type) != sem.KRef
}
