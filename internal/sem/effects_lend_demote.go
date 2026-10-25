package sem

import "kigumi/internal/syntax"

// DemoteBoxedLend records a lend the built program could not keep direct, so the body that wrote it allocates after all.
func (r *Result) DemoteBoxedLend(file FileID, node syntax.NodeID) {
	site, ok := r.lendSites[lendSite{file, node}]
	if ok && !r.boxedLends[site] {
		r.pendingLends = append(r.pendingLends, site)
	}
}

// SettleLendDemotions re-infers the summaries without the demoted lends and charges every declared
// `noalloc` caller that relied on one.
func (r *Result) SettleLendDemotions() {
	if len(r.pendingLends) == 0 {
		return
	}
	nodes := r.effectNodes()
	before := r.noallocStates(nodes)
	if r.boxedLends == nil {
		r.boxedLends = map[lendSite]bool{}
	}
	for _, s := range r.pendingLends {
		r.boxedLends[s] = true
	}
	r.pendingLends = nil
	r.inferSummaries(nodes)
	for _, id := range nodes {
		declared := r.Entities[id].Kind != EntClosure && r.Fn(id).Declared&EffNoalloc != 0
		for i, e := range r.edgesOf(id) {
			switch {
			case e.Kind == EdgeContract:
				if e.Mods&EffNoalloc != 0 && before.summary[e.Target] && !r.summary(e.Target).Noalloc {
					r.checkContractEdge(e, EffNoalloc)
				}
			case declared && before.edges[id][i] && !r.edgeAllows(e, EffNoalloc):
				r.reportEdge(e, EffNoalloc)
			}
		}
	}
	if r.noDefault || r.recoverableOnly {
		r.recheckProfiles(nodes)
	}
}

// profileDeferred: the built program answers for these allocations.
func (r *Result) profileDeferred(e EffectEdge) bool {
	return r.undemoted(e) || r.takenUnboxed(e) || witnessWhy(e.Why)
}

// demotableWhy: the built program demotes the sites where these allocations stay.
func demotableWhy(why uint8) bool { return why == whyLendCross || why == whyWitnessBind }

func (r *Result) undemoted(e EffectEdge) bool {
	return demotableWhy(e.Why) && !r.boxedLends[lendSite{e.File, e.Node}]
}

// ProfileTrusts reports whether the active allocation profile accepted fn, possibly on crossings it left to the built program.
func (r *Result) ProfileTrusts(fn EntityID) bool {
	switch {
	case r.recoverableOnly:
		return r.recoverable.safe[fn]
	case r.noDefault:
		return r.defaultOf(fn)
	}
	return false
}

type noallocStates struct {
	summary map[EntityID]bool
	edges   map[EntityID][]bool
}

func (r *Result) noallocStates(nodes []EntityID) noallocStates {
	st := noallocStates{summary: map[EntityID]bool{}, edges: map[EntityID][]bool{}}
	for id := 1; id < len(r.Entities); id++ {
		if k := r.Entities[id].Kind; k == EntFn || k == EntClosure || k == EntContract || k == EntImplicitMain || k == EntTest {
			st.summary[EntityID(id)] = r.summary(EntityID(id)).Noalloc
		}
	}
	for _, id := range nodes {
		for _, e := range r.edgesOf(id) {
			st.edges[id] = append(st.edges[id], r.edgeAllows(e, EffNoalloc))
		}
	}
	return st
}
