package sem

import "slices"

// ifaceReqImpls pools every witness conformMemo ever recorded for a
// requirement; ifaceCallHops narrows this per call site where it can.
func (r *Result) ifaceReqImpls() map[EntityID][]EntityID {
	out := map[EntityID][]EntityID{}
	for key, res := range r.conformMemo {
		if !res.ok {
			continue
		}
		reqs := r.iface(r.Types.Node(key.iface).Ent).Reqs
		for i, w := range res.witnesses {
			if i < len(reqs) && !slices.Contains(out[reqs[i]], w) {
				out[reqs[i]] = append(out[reqs[i]], w)
			}
		}
	}
	for req := range out {
		slices.Sort(out[req])
	}
	return out
}

// rootRunImpl mirrors rootRunBind: a requirement's distance is the best
// distance among the witnesses that might be its runtime receiver.
func (r *Result) rootRunImpl(req EntityID, impls []EntityID, rr *rootRun) bool {
	best, via := 0, EntityID(0)
	for _, w := range impls {
		if d := rr.hops(w); d > 0 && (best == 0 || d < best) {
			best, via = d, w
		}
	}
	if best > 0 && (rr.dist[req] == 0 || best < rr.dist[req]) {
		rr.dist[req], rr.witness[req] = best, via
		return true
	}
	return false
}
