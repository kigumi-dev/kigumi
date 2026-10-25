package sem

// recoverableProfile is the recoverable-only profile's state: the converged safety, every witness site,
// and per generic fn the witness slots it calls a requirement through.
type recoverableProfile struct {
	safe    map[EntityID]bool
	sites   []pendingWitnessCallback
	used    map[EntityID][]bool
	byOwner map[EntityID][]int
}

// boundLocal is the witness slot's local a requirement call dispatches through, 0 for an interface value.
func (r *Result) boundLocal(e EffectEdge) EntityID {
	if e.Kind != EdgeCall && e.Kind != EdgeWitnessCall || e.Target == 0 {
		return 0
	}
	if _, ok := r.boundContract(e.Target); !ok {
		return 0
	}
	call := r.Files[e.File].Calls[e.Node]
	if call.Callee != e.Target || call.Witness == 0 {
		return 0
	}
	if _, k := r.witnessSlotOf(call.Witness); k < 0 {
		return 0
	}
	return call.Witness
}

func (r *Result) witnessSlotOf(local EntityID) (EntityID, int) {
	fn := r.Entities[local].Parent
	if r.Entities[fn].Kind != EntFn {
		return 0, -1
	}
	for k, w := range r.Fn(fn).Witnesses {
		if w.Local == local {
			return fn, k
		}
	}
	return 0, -1
}

// markBoundSlots finds the witness slots each generic fn calls through, directly or via a callee it
// hands the witness to, so every site instantiating the fn can judge the impl it passes.
func (r *Result) markBoundSlots(nodes []EntityID) {
	used := map[EntityID][]bool{}
	mark := func(local EntityID) bool {
		fn, k := r.witnessSlotOf(local)
		if k < 0 || used[fn] != nil && used[fn][k] {
			return false
		}
		if used[fn] == nil {
			used[fn] = make([]bool, len(r.Fn(fn).Witnesses))
		}
		used[fn][k] = true
		return true
	}
	for _, id := range nodes {
		for _, e := range r.edgesOf(id) {
			if local := r.boundLocal(e); local != 0 {
				mark(local)
			}
		}
	}
	var markArg func(a WitnessArg) bool
	markArg = func(a WitnessArg) bool {
		switch a.Kind {
		case WitnessLocal:
			return mark(a.Ent)
		case WitnessBind:
			changed := false
			for j, u := range used[a.Ent] {
				if u && j < len(a.Args) && markArg(a.Args[j]) {
					changed = true
				}
			}
			return changed
		}
		return false
	}
	owners := map[EntityID][]int{}
	for i, s := range r.recoverable.sites {
		owners[s.owner] = append(owners[s.owner], i)
	}
	for changed := true; changed; {
		changed = false
		for _, s := range r.recoverable.sites {
			for i, u := range used[s.fn] {
				if u && i < len(s.passes) && markArg(s.passes[i]) {
					changed = true
				}
			}
		}
	}
	r.recoverable.used, r.recoverable.byOwner = used, owners
}

// badWitness is the first witness a site hands fn for a slot fn calls through that the profile rejects:
// the impl that would run (0 when unresolved) and the requirement it stands for.
func (r *Result) badWitness(fn EntityID, passes []WitnessArg, safe map[EntityID]bool) (EntityID, EntityID, bool) {
	for i, u := range r.recoverable.used[fn] {
		if !u {
			continue
		}
		req := r.Fn(fn).Witnesses[i].Req
		if i >= len(passes) {
			return 0, req, true
		}
		if impl, at, bad := r.badPass(passes[i], req, safe); bad {
			return impl, at, true
		}
	}
	return 0, 0, false
}

func (r *Result) badPass(a WitnessArg, req EntityID, safe map[EntityID]bool) (EntityID, EntityID, bool) {
	switch a.Kind {
	case WitnessLocal:
		_, k := r.witnessSlotOf(a.Ent)
		return 0, req, k < 0
	case WitnessFn, WitnessBind:
		if !r.recoverableTargetSafe(a.Ent, safe) {
			return a.Ent, req, true
		}
		if a.Kind == WitnessBind {
			return r.badWitness(a.Ent, a.Args, safe)
		}
		return 0, 0, false
	}
	return 0, req, true
}

// badSiteOf is the first witness site in owner's body whose witness the profile rejects.
func (r *Result) badSiteOf(owner EntityID, safe map[EntityID]bool) (pendingWitnessCallback, EntityID, EntityID, bool) {
	for _, i := range r.recoverable.byOwner[owner] {
		s := r.recoverable.sites[i]
		if impl, req, bad := r.badWitness(s.fn, s.passes, safe); bad {
			return s, impl, req, true
		}
	}
	return pendingWitnessCallback{}, 0, 0, false
}
