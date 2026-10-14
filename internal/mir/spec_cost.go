package mir

// The shared body boxes a runtime argument (a constant's box is a free immortal) and returns a box.
func (s *specializer) revertCostly(sp *spec, fixed map[*Func][]bool) bool {
	g := s.p.ByEnt[sp.gen]
	changed := false
	for _, site := range sp.sites {
		if site.gone {
			continue
		}
		c := site.f
		if fixed[c] == nil {
			fixed[c] = s.p.fixedLocals(c)
		}
		in := c.Blocks[site.blk].Insts[site.idx]
		saved := 0
		for i, a := range in.Args {
			if sp.fn.DSig.Param(i) != 0 && g.DSig.Param(i) == 0 && c.IsDirect(a) && !fixed[c][a] {
				saved++
			}
		}
		added := 0
		if sp.fn.DSig.result() != 0 && g.DSig.result() == 0 {
			added = s.reboxes(c, in)
		}
		if added > saved {
			s.revert(sp, site)
			changed = true
		}
	}
	return changed
}

func (s *specializer) reboxes(c *Func, call Inst) int {
	n := 0
	if cross, _, ok := s.p.dstCross(c, call); ok && cross == OpBoxDirect {
		n++
	}
	from := map[LocalID]bool{call.Dst: true}
	for changed := true; changed; {
		changed = false
		for _, blk := range c.Blocks {
			for _, in := range blk.Insts {
				switch in.Op {
				case OpCopy, OpAlias, OpMove, OpShare:
					if from[in.Args[0]] && !from[in.Dst] {
						from[in.Dst], changed = true, true
					}
				}
			}
		}
	}
	for _, blk := range c.Blocks {
		for _, in := range blk.Insts {
			for i, a := range in.Args {
				if cross, _, ok := s.p.argCross(c, in, i, a); ok && from[a] && cross == OpBoxDirect {
					n++
				}
			}
		}
		if blk.Term.Op == TermReturn && from[blk.Term.Args[0]] {
			if cross, _, ok := s.p.returnCross(c, blk.Term.Args[0]); ok && cross == OpBoxDirect {
				n++
			}
		}
	}
	return n
}

// devirtualizes reports an instance calling directly a witness its shared body reaches through a box.
func (s *specializer) devirtualizes(pairs []funcPair) bool {
	for _, pr := range pairs {
		if s.witnessBoxes(pr.fn) < s.witnessBoxes(pr.gen) {
			return true
		}
	}
	return false
}

func (s *specializer) witnessBoxes(f *Func) int {
	n := 0
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op != OpCallValue || in.Ent == 0 {
				continue
			}
			for _, a := range in.Args[1:] {
				if s.p.R.BoxAllocates(f.Locals[a].Type) {
					n++
				}
			}
			if s.p.R.BoxAllocates(f.Locals[in.Dst].Type) {
				n++
			}
		}
	}
	return n
}
