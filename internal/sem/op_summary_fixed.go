package sem

// opFixedSummary is OpSummary's analogue of fixedSummary: a bodiless function with a reviewed contract
// takes the contract's facts; any other one is Untrusted, its declared `noalloc` clearing only FactAlloc.
func (r *Result) opFixedSummary(id EntityID) (OpSummary, bool) {
	e := &r.Entities[id]
	if e.Kind == EntClosure {
		return OpSummary{}, false
	}
	if e.Kind != EntFn && e.Kind != EntContract && e.Kind != EntImplicitMain && e.Kind != EntTest {
		return OpSummary{Known: true}, true
	}
	info := r.Fn(id)
	if info.Body != 0 {
		return OpSummary{}, false
	}
	if s, ok := trustedOpSummaries[r.stdKey(id)]; ok {
		return s, true
	}
	if c, ok := r.isrContractOf(id); ok {
		return contractSummary(c), true
	}
	s := OpSummary{Known: true, Untrusted: true}
	trusted := info.Declared&EffNoalloc != 0
	s.Facts[FactAlloc] = OpFactState{Maybe: !trusted}
	for f := OpFact(0); f < numOpFacts; f++ {
		if f == FactAlloc || trusted {
			continue
		}
		s.Facts[f] = OpFactState{Maybe: true, Cause: OpCause{Unknown: true}}
	}
	if directBlockingPrimitives[r.stdKey(id)] {
		s.Facts[FactBlock] = OpFactState{Maybe: true}
	}
	return s, true
}

func contractSummary(c IsrContract) OpSummary {
	s := OpSummary{Known: true}
	switch c.Kind {
	case IsrSync:
		s.Facts[FactSync] = OpFactState{Maybe: true, Contract: true}
		s.Contracts = c.ID
	case IsrMMIO:
		s.Facts[FactIO] = OpFactState{Maybe: true, Contract: true}
		s.Contracts = c.ID
	}
	return s
}

func unionContracts(a, b string) string {
	switch {
	case b == "" || a == b:
		return a
	case a == "":
		return b
	}
	set := map[string]bool{}
	for _, s := range []string{a, b} {
		for _, id := range splitContracts(s) {
			set[id] = true
		}
	}
	return joinContracts(set)
}
