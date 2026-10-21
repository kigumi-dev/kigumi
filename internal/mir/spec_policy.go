package mir

// A revert changes its caller's crossings, so the check repeats until stable.
func (s *specializer) settle() {
	live := s.live()
	if len(live) == 0 {
		return
	}
	fs := s.p.allFuncs()
	pins := make(map[*Func][]bool, len(fs))
	for _, f := range fs {
		pins[f] = s.p.pinned(f)
		f.DSig = s.p.directSig(f, pins[f])
	}
	s.lent = s.p.lendCandidates()
	for len(live) > 0 && s.dryRun(live, pins) {
		live = s.live()
	}
	for _, f := range fs {
		f.DSig, f.Direct = nil, nil
		for bid := range f.Blocks {
			for i := range f.Blocks[bid].Insts {
				f.Blocks[bid].Insts[i].DPrim = DPNone
			}
		}
	}
}

// dryRun classifies like plan-direct without writing a crossing.
func (s *specializer) dryRun(live []*spec, pins map[*Func][]bool) bool {
	for _, f := range s.callers(live) {
		s.p.markDirectPrims(f)
		f.Direct = s.p.classifyDirect(f, pins[f])
		s.p.settlePrims(f, pins[f])
	}
	changed := false
	s.reboxFlows = nil
	fixed := map[*Func][]bool{}
	held := s.noallocHeld(live)
	for _, sp := range live {
		changed = s.revertCostly(sp, fixed) || changed
		if !s.pays(sp) || s.deepens(sp, held) {
			s.drop(sp)
			changed = true
		}
	}
	return changed
}

func (s *specializer) callers(live []*spec) []*Func {
	seen := map[*Func]bool{}
	var out []*Func
	add := func(f *Func) {
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	for _, sp := range live {
		for _, f := range sp.funcs() {
			add(f)
		}
		for _, site := range sp.sites {
			if !site.gone {
				add(site.f)
			}
		}
	}
	return out
}

// The shared body never boxes a type-parameter value, and callers rely on a `noalloc` summary.
func (s *specializer) pays(sp *spec) bool {
	if sp.fn.Proto != "" {
		return true
	}
	gains := false
	if sp.scalar {
		pays, inReg := s.scalarPays(sp)
		if !pays {
			return false
		}
		gains = inReg
	}
	pairs := append([]funcPair{{sp.fn, s.p.ByEnt[sp.gen]}}, sp.clones...)
	for _, pr := range pairs {
		f, g := pr.fn, pr.gen
		derived := func(l LocalID) bool { return int(l) < len(g.Locals) && g.Locals[l].Type != f.Locals[l].Type }
		for i, prm := range f.Params {
			gains = gains || (f.DSig.Param(i) != 0 || s.lent[lendParam{f, i}]) && derived(prm)
		}
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				gains = gains || builds(in) && derived(in.Dst) && f.IsDirect(in.Dst) && s.p.dstPos(f, in) != 0
			}
		}
	}
	if !gains && !s.devirtualizes(pairs) {
		return false
	}
	for _, pr := range pairs {
		f, g := pr.fn, pr.gen
		noalloc := s.p.R.Summary(g.Ent).Noalloc
		derived := func(l LocalID) bool { return int(l) < len(g.Locals) && g.Locals[l].Type != f.Locals[l].Type }
		counted := func(l LocalID) bool { return noalloc || derived(l) }
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				for i, a := range in.Args {
					if cross, _, ok := s.p.argCross(f, in, i, a); ok && cross == OpBoxDirect && counted(a) {
						return false
					}
				}
				if cross, _, ok := s.p.dstCross(f, in); ok && cross == OpBoxDirect && counted(in.Dst) {
					return false
				}
			}
			if blk.Term.Op == TermReturn {
				a := blk.Term.Args[0]
				if cross, _, ok := s.p.returnCross(f, a); ok && cross == OpBoxDirect && counted(a) {
					return false
				}
			}
		}
	}
	return true
}

func (s *specializer) drop(sp *spec) {
	sp.dead = true
	for _, site := range sp.sites {
		if !site.gone {
			s.revert(sp, site)
		}
	}
}

func (s *specializer) revert(sp *spec, site *callSite) {
	site.gone = true
	delete(s.reboxFlows, site.f)
	site.f.Blocks[site.blk].Insts[site.idx].Ent = sp.gen
}

func (s *specializer) live() []*spec {
	reached := map[*spec]bool{}
	var out []*spec
	for changed := true; changed; {
		changed = false
		for _, sp := range s.all {
			if sp.dead || reached[sp] || !s.called(sp, reached) {
				continue
			}
			reached[sp], changed = true, true
			out = append(out, sp)
		}
	}
	return out
}

func (s *specializer) called(sp *spec, reached map[*spec]bool) bool {
	for _, site := range sp.sites {
		if site.gone {
			continue
		}
		if caller := s.byFunc[site.f]; caller == nil || reached[caller] {
			return true
		}
	}
	return false
}

func (s *specializer) prune() {
	keep := map[*Func]bool{}
	for _, sp := range s.live() {
		for _, f := range sp.funcs() {
			keep[f] = true
		}
	}
	out := s.p.Funcs[:0]
	for _, f := range s.p.Funcs {
		if sp := s.byFunc[f]; sp != nil && !keep[f] {
			delete(s.p.ByEnt, f.Ent)
			continue
		}
		out = append(out, f)
	}
	s.p.Funcs = out
}

func builds(in Inst) bool {
	switch in.Op {
	case OpRecord, OpClosure, OpCall:
		return true
	case OpVariant:
		return len(in.Args) > 0
	}
	return false
}
