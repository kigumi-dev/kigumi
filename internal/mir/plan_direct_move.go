package mir

func (f *Func) moveLast() {
	for b := range f.Blocks {
		for k := range f.Blocks[b].Insts {
			in := &f.Blocks[b].Insts[k]
			if in.Op != OpShare || !in.Last || f.IsDirect(in.Args[0]) || f.IsDirect(in.Dst) {
				continue
			}
			in.Op, in.Copy, in.IfWritable, in.Own = OpMove, false, false, false
		}
	}
}
