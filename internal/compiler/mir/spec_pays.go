package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// A held instance stays without a gain; one that boxes where the shared body does not is still dropped.
func (s *specializer) pays(sp *spec) bool {
	if sp.fn.Proto != "" {
		return true
	}
	if sp.held == nil && !s.gains(sp) {
		return false
	}
	return !s.boxesMore(sp)
}

func (s *specializer) gains(sp *spec) bool {
	gains := false
	if sp.scalar {
		pays, inReg := s.scalarPays(sp)
		if !pays {
			return false
		}
		gains = inReg
	}
	pairs := s.pairs(sp)
	for _, pr := range pairs {
		f, g := pr.fn, pr.gen
		derived := func(l midir.LocalID) bool { return int(l) < len(g.Locals) && g.Locals[l].Type != f.Locals[l].Type }
		for i, prm := range f.Params {
			gains = gains || (f.DSig.Param(i) != 0 || s.lent[lendParam{f, i}]) && derived(prm)
		}
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				gains = gains || builds(in) && derived(in.Dst) && f.IsDirect(in.Dst) && s.p.dstPos(f, in) != 0
			}
		}
	}
	return gains || s.devirtualizes(pairs)
}

func (s *specializer) pairs(sp *spec) []funcPair {
	return append([]funcPair{{sp.fn, s.p.ByEnt[sp.gen]}}, sp.clones...)
}

// The shared body never boxes a type-parameter value, and callers rely on a `noalloc` summary.
func (s *specializer) boxesMore(sp *spec) bool {
	for _, pr := range s.pairs(sp) {
		f, g := pr.fn, pr.gen
		noalloc := s.p.R.Summary(g.Ent).Noalloc
		derived := func(l midir.LocalID) bool { return int(l) < len(g.Locals) && g.Locals[l].Type != f.Locals[l].Type }
		counted := func(l midir.LocalID) bool { return noalloc || derived(l) }
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				for i, a := range in.Args {
					if cross, _, ok := s.p.argCross(f, in, i, a); ok && cross == midir.OpBoxDirect && counted(a) {
						return true
					}
				}
				if cross, _, ok := s.p.dstCross(f, in); ok && cross == midir.OpBoxDirect && counted(in.Dst) {
					return true
				}
			}
			if blk.Term.Op == midir.TermReturn {
				a := blk.Term.Args[0]
				if cross, _, ok := s.p.returnCross(f, a); ok && cross == midir.OpBoxDirect && counted(a) {
					return true
				}
			}
		}
	}
	return false
}
