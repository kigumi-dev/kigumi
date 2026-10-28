package sem

import "slices"

// witnessSeal is what resolveRefCells settled: how many conformance queries were asked, and the witness table by
// requirement, nil before the seal.
type witnessSeal struct {
	conforms  int
	witnesses map[EntityID][]EntityID
}

// SealedWitnesses are the methods the witness table names for interface requirement req as resolveRefCells sealed it;
// ok is false before the seal, so a backend never asks the conformance check late.
func (r *Result) SealedWitnesses(req EntityID) ([]EntityID, bool) {
	if r.seal.witnesses == nil {
		return nil, false
	}
	return r.seal.witnesses[req], true
}

func (r *Result) witnessesByRequirement() map[EntityID][]EntityID {
	out := map[EntityID][]EntityID{}
	for _, p := range r.witnessPairs {
		out[p.req] = append(out[p.req], p.m)
	}
	for _, ws := range out {
		slices.Sort(ws)
	}
	return out
}
