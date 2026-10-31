package mir

func (a *owner) stateAt(bid BlockID, idx int) *dstate {
	if a.in[bid] == nil {
		return nil
	}
	blk := &a.f.Blocks[bid]
	saved := *blk
	blk.Insts, blk.Term = saved.Insts[:idx:idx], Term{}
	st := a.transfer(bid, cloneD(a.in[bid]), false)
	*blk = saved
	return st
}

func (a *owner) solveGhosts() {
	for _, g := range a.f.ghosts.runs {
		for _, m := range g.marks {
			if st := a.stateAt(m.blk, m.idx); st != nil {
				a.merge(g.entry, st)
			}
		}
		if a.in[g.entry] != nil {
			a.solve(g.entry)
		}
	}
}
