package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// The shared body boxes a runtime argument (a constant's box is a free immortal) and returns a box.
func (s *specializer) revertCostly(sp *spec, fixed map[*midir.Func][]bool) bool {
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
		if sp.fn.DSig.Result() != 0 && g.DSig.Result() == 0 {
			added = s.reboxes(c, in)
		}
		if added > saved {
			s.revert(sp, site)
			changed = true
		}
	}
	return changed
}

type reboxFlow struct {
	copies map[midir.LocalID][]midir.LocalID
	boxed  map[midir.LocalID]int
}

func (s *specializer) reboxFlowOf(c *midir.Func) *reboxFlow {
	if fl := s.reboxFlows[c]; fl != nil {
		return fl
	}
	fl := &reboxFlow{copies: map[midir.LocalID][]midir.LocalID{}, boxed: map[midir.LocalID]int{}}
	for _, blk := range c.Blocks {
		for _, in := range blk.Insts {
			switch in.Op {
			case midir.OpCopy, midir.OpAlias, midir.OpMove, midir.OpShare:
				fl.copies[in.Args[0]] = append(fl.copies[in.Args[0]], in.Dst)
			}
			for i, a := range in.Args {
				if cross, _, ok := s.p.argCross(c, in, i, a); ok && cross == midir.OpBoxDirect {
					fl.boxed[a]++
				}
			}
		}
		if blk.Term.Op == midir.TermReturn {
			if cross, _, ok := s.p.returnCross(c, blk.Term.Args[0]); ok && cross == midir.OpBoxDirect {
				fl.boxed[blk.Term.Args[0]]++
			}
		}
	}
	if s.reboxFlows == nil {
		s.reboxFlows = map[*midir.Func]*reboxFlow{}
	}
	s.reboxFlows[c] = fl
	return fl
}

func (s *specializer) reboxes(c *midir.Func, call midir.Inst) int {
	n := 0
	if cross, _, ok := s.p.dstCross(c, call); ok && cross == midir.OpBoxDirect {
		n++
	}
	fl := s.reboxFlowOf(c)
	from := map[midir.LocalID]bool{call.Dst: true}
	for work := []midir.LocalID{call.Dst}; len(work) > 0; {
		l := work[len(work)-1]
		work = work[:len(work)-1]
		n += fl.boxed[l]
		for _, d := range fl.copies[l] {
			if !from[d] {
				from[d] = true
				work = append(work, d)
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

func (s *specializer) witnessBoxes(f *midir.Func) int {
	n := 0
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op != midir.OpCallValue || in.Ent == 0 {
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
