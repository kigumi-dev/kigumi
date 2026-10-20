package mir

// The Future holds a reference to every argument, so the body returns the borrowed ones when it finishes.
func (p *Program) completeReturns(f *Func) {
	for bid := range f.Blocks {
		blk := &f.Blocks[bid]
		switch {
		case blk.Term.Op == TermReturn:
			blk.Insts = append(blk.Insts, p.lentReleases(f)...)
			blk.Insts = append(blk.Insts, Inst{Op: OpFutureDone, Args: []LocalID{blk.Term.Args[0]}})
		case endsCancelled(blk):
			last := len(blk.Insts) - 1
			tail := append(p.lentReleases(f), blk.Insts[last])
			blk.Insts = append(blk.Insts[:last], tail...)
		}
	}
}

func endsCancelled(blk *Block) bool {
	n := len(blk.Insts)
	return n > 0 && blk.Insts[n-1].Op == OpCancelDone
}

func (p *Program) lentReleases(f *Func) []Inst {
	var out []Inst
	for i, prm := range f.Params {
		if p.CaptureLent(f, i) {
			out = append(out, Inst{Op: OpRelease, Args: []LocalID{prm}})
		}
	}
	return out
}
