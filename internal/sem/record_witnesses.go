package sem

type witnessPair struct{ req, m EntityID }

// Only a resolved pair reaches the callee's convention; a later query repeats a key already settled here.
func (r *Result) noteWitness(req, m EntityID) {
	p := witnessPair{req: req, m: m}
	if req == 0 || m == 0 || req == m || r.witnessSeen[p] {
		return
	}
	r.witnessSeen[p] = true
	r.witnessPairs = append(r.witnessPairs, p)
}

func (r *Result) witnessesOf(req EntityID) []EntityID {
	var out []EntityID
	for _, p := range r.witnessPairs {
		if p.req == req {
			out = append(out, p.m)
		}
	}
	return out
}
