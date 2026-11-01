package ownership

import midir "kigumi/internal/compiler/mir/ir"

func (a *owner) stateAt(bid midir.BlockID, idx int) *dstate {
	if a.in[bid] == nil {
		return nil
	}
	blk := &a.f.Blocks[bid]
	saved := *blk
	blk.Insts, blk.Term = saved.Insts[:idx:idx], midir.Term{}
	st := a.transfer(bid, cloneD(a.in[bid]), false)
	*blk = saved
	return st
}

func (a *owner) solveGhosts() {
	for _, g := range a.ghosts {
		for _, m := range g.Marks {
			if st := a.stateAt(m.Blk, m.Idx); st != nil {
				a.merge(g.Entry, st)
			}
		}
		if a.in[g.Entry] != nil {
			a.solve(g.Entry)
		}
	}
}
