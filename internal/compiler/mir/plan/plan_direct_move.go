package plan

import midir "kigumi/internal/compiler/mir/ir"

func settleElided(f *midir.Func) {
	for b := range f.Blocks {
		for k := range f.Blocks[b].Insts {
			in := &f.Blocks[b].Insts[k]
			if in.Op != midir.OpShare || !(in.Last || in.Retain) || f.IsDirect(in.Args[0]) || f.IsDirect(in.Dst) {
				continue
			}
			if in.Last {
				in.Op = midir.OpMove
			}
			in.Copy, in.IfWritable, in.Own = false, false, false
		}
	}
}
