package mir

import "slices"

// deleteInst removes block b's instruction i; a ghost mark keeps standing before the instruction it stood before.
func (f *Func) deleteInst(b, i int) Inst {
	in := f.Blocks[b].Insts[i]
	f.Blocks[b].Insts = slices.Delete(f.Blocks[b].Insts, i, i+1)
	f.shiftMarks(b, i+1, -1)
	return in
}

func (f *Func) insertInst(b, i int, in Inst) {
	f.Blocks[b].Insts = slices.Insert(f.Blocks[b].Insts, i, in)
	f.shiftMarks(b, i, 1)
}

func (f *Func) shiftMarks(b, from, d int) {
	for r := range f.ghosts.runs {
		for k, m := range f.ghosts.runs[r].marks {
			if int(m.blk) == b && m.idx >= from {
				f.ghosts.runs[r].marks[k].idx += d
			}
		}
	}
}

// ghostRange is the blocks [start, end) holding the ghost runs, empty when there are none.
func (f *Func) ghostRange() (int, int) {
	if len(f.ghosts.runs) == 0 {
		return len(f.Blocks), len(f.Blocks)
	}
	return f.ghosts.blocks, int(f.ghosts.runs[len(f.ghosts.runs)-1].end)
}
