package mir

func (f *Func) settleElided() {
	for b := range f.Blocks {
		for k := range f.Blocks[b].Insts {
			in := &f.Blocks[b].Insts[k]
			if in.Op != OpShare || !(in.Last || in.Retain) || f.IsDirect(in.Args[0]) || f.IsDirect(in.Dst) {
				continue
			}
			if in.Last {
				in.Op = OpMove
			}
			in.Copy, in.IfWritable, in.Own = false, false, false
		}
	}
}
