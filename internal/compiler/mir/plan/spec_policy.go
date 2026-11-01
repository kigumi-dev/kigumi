package plan

import (
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
)

// A revert changes its caller's crossings, so the check repeats until stable.
func (s *specializer) settle() {
	live := s.live()
	if len(live) == 0 {
		return
	}
	fs := s.p.AllFuncs()
	pins := make(map[*midir.Func][]bool, len(fs))
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
				f.Blocks[bid].Insts[i].DPrim = opdef.DPNone
			}
		}
	}
}

// dryRun classifies like plan-direct without writing a crossing.
func (s *specializer) dryRun(live []*spec, pins map[*midir.Func][]bool) bool {
	for _, f := range s.callers(live) {
		s.p.markDirectPrims(f)
		f.Direct = s.p.classifyDirect(f, pins[f])
		s.p.settlePrims(f, pins[f])
	}
	changed := false
	s.reboxFlows = nil
	fixed := map[*midir.Func][]bool{}
	for _, sp := range live {
		changed = s.revertCostly(sp, fixed) || changed
		if !s.pays(sp) || s.deepens(sp) {
			s.drop(sp)
			changed = true
		}
	}
	return changed
}

func (s *specializer) callers(live []*spec) []*midir.Func {
	seen := map[*midir.Func]bool{}
	var out []*midir.Func
	add := func(f *midir.Func) {
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
	keep := map[*midir.Func]bool{}
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

func builds(in midir.Inst) bool {
	switch in.Op {
	case midir.OpRecord, midir.OpClosure, midir.OpCall:
		return true
	case midir.OpVariant:
		return len(in.Args) > 0
	}
	return false
}
