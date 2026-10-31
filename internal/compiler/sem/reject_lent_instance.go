package sem

import "slices"

// lentInstanceKeepsValue: the callee takes a cell only because another instantiation of its type parameter replaces the referent.
// Anything it cannot read off the call site (a chain of callees, an open type, a conformance nobody queried) stays rejected.
func (r *Result) lentInstanceKeepsValue(v refCellValue) bool {
	if r.refCellHard[v.param] || r.Types.ContainsVar(v.referent) || len(r.Types.Params(v.referent)) > 0 {
		return false
	}
	for _, e := range r.refCellReach(v.param) {
		if e != v.param && r.hasWitnessRecv(e) {
			return false
		}
	}
	from := r.packageOf(v.file)
	for _, q := range r.refCellRecvs {
		if q.ent != v.param || !q.witness {
			continue
		}
		if r.Types.ContainsVar(q.iface) || len(r.Types.Params(q.iface)) > 0 {
			return false
		}
		res, queried := r.conformMemo[conformKey{v.referent, q.iface, from}]
		idx := slices.Index(r.iface(r.Types.Node(q.iface).Ent).Reqs, q.callee)
		if !queried || idx < 0 {
			return false
		}
		if res.ok && idx < len(res.witnesses) && r.replacesSelf(res.witnesses[idx]) {
			return false
		}
	}
	return true
}

func (r *Result) hasWitnessRecv(ent EntityID) bool {
	for _, q := range r.refCellRecvs {
		if q.ent == ent && q.witness {
			return true
		}
	}
	return false
}

// refCellReach lists the bindings whose cell would force one at ent, ent included.
func (r *Result) refCellReach(ent EntityID) []EntityID {
	out, seen := []EntityID{ent}, map[EntityID]bool{ent: true}
	for i := 0; i < len(out); i++ {
		for _, e := range r.refCellEdgeList {
			if e.from == out[i] && !seen[e.to] {
				seen[e.to] = true
				out = append(out, e.to)
			}
		}
	}
	return out
}
