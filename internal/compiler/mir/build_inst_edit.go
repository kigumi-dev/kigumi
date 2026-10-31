package mir

import "slices"

// deleteInst removes block b's instruction i; a ghost mark keeps standing before the instruction it stood before.
func (g *ghostSet) deleteInst(f *Func, b, i int) Inst {
	in := f.Blocks[b].Insts[i]
	f.Blocks[b].Insts = slices.Delete(f.Blocks[b].Insts, i, i+1)
	g.shiftMarks(b, i+1, -1)
	return in
}

func (g *ghostSet) insertInst(f *Func, b, i int, in Inst) {
	f.Blocks[b].Insts = slices.Insert(f.Blocks[b].Insts, i, in)
	g.shiftMarks(b, i, 1)
}

func (g *ghostSet) shiftMarks(b, from, d int) {
	for r := range g.runs {
		for k, m := range g.runs[r].marks {
			if int(m.blk) == b && m.idx >= from {
				g.runs[r].marks[k].idx += d
			}
		}
	}
}

// ghostRange is the blocks [start, end) holding the ghost runs, empty when there are none.
func (g *ghostSet) ghostRange(f *Func) (int, int) {
	if len(g.runs) == 0 {
		return len(f.Blocks), len(f.Blocks)
	}
	return g.blocks, int(g.runs[len(g.runs)-1].end)
}
