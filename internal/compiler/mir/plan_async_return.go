package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// The Future holds a reference to every argument, so the body returns the borrowed ones when it finishes.
func (p *Program) completeReturns(f *midir.Func) {
	for bid := range f.Blocks {
		blk := &f.Blocks[bid]
		switch {
		case blk.Term.Op == midir.TermReturn:
			blk.Insts = append(blk.Insts, p.lentReleases(f)...)
			blk.Insts = append(blk.Insts, midir.Inst{Op: midir.OpFutureDone, Args: []midir.LocalID{blk.Term.Args[0]}})
		case endsCancelled(blk):
			last := len(blk.Insts) - 1
			tail := append(p.lentReleases(f), blk.Insts[last])
			blk.Insts = append(blk.Insts[:last], tail...)
		}
	}
}

func endsCancelled(blk *midir.Block) bool {
	n := len(blk.Insts)
	return n > 0 && blk.Insts[n-1].Op == midir.OpCancelDone
}

func (p *Program) lentReleases(f *midir.Func) []midir.Inst {
	var out []midir.Inst
	for i, prm := range f.Params {
		if p.CaptureLent(f, i) {
			out = append(out, midir.Inst{Op: midir.OpRelease, Args: []midir.LocalID{prm}})
		}
	}
	return out
}
