package sem

import "slices"

// dropOpSummary is dropEffect's generalization to all facts; it keeps its
// own memo so neither system's cache affects the other.
func (r *Result) dropOpSummary(t TypeID) OpSummary {
	neutral := OpSummary{Known: true}
	if t == 0 {
		return neutral
	}
	if s, ok := r.opDropMemo[t]; ok {
		return s
	}
	r.opDropMemo[t] = neutral
	s := r.computeDropOp(t)
	r.opDropMemo[t] = s
	return s
}

func unknownOp() OpSummary {
	s := OpSummary{Known: true, Unresolved: true}
	for f := OpFact(0); f < numOpFacts; f++ {
		s.Facts[f] = OpFactState{Maybe: true, Cause: OpCause{Unknown: true}}
	}
	return s
}

func (r *Result) computeDropOp(t TypeID) OpSummary {
	tt := r.Types
	n := tt.Node(t)
	switch n.Kind {
	case KIface, KParam:
		return unknownOp()
	case KClosure:
		if n.Ent == 0 {
			return OpSummary{Known: true}
		}
		s := OpSummary{Known: true}
		for _, cap := range r.closure(n.Ent).Captures {
			s = mergeOp(s, r.dropOpSummary(r.Entities[cap.Local].Type))
		}
		return s
	case KNamed:
	default:
		return OpSummary{Known: true}
	}
	info := r.typeDecl(n.Ent)
	s := OpSummary{Known: true}
	if info.Form == FormResource && info.Drop != 0 {
		s = mergeOp(s, r.opSummary(info.Drop))
	}
	if r.isHeapCell(n.Ent) {
		s.Facts[FactRelease] = OpFactState{Maybe: true}
	}
	subst := map[EntityID]TypeID{}
	for i, p := range info.Params {
		if i < len(n.Args) {
			subst[p] = n.Args[i]
		}
	}
	for _, a := range n.Args {
		s = mergeOp(s, r.dropOpSummary(a))
	}
	for _, f := range info.Fields {
		s = mergeOp(s, r.dropOpSummary(tt.Subst(r.Entities[f].Type, subst)))
	}
	for _, v := range info.Variants {
		for _, p := range r.variant(v).Payload {
			s = mergeOp(s, r.dropOpSummary(tt.Subst(p, subst)))
		}
	}
	return s
}

// isAllocType matches by name+package: these std/alloc types have no langItemTable entry.
func (r *Result) isAllocType(ent EntityID, names ...string) bool {
	alloc, ok := r.pathIndex["std/alloc"]
	return ok && r.Entities[ent].Pkg == alloc && slices.Contains(names, r.Entities[ent].Name)
}

// isHeapCell reports whether dropping a value of this type frees a heap cell.
func (r *Result) isHeapCell(ent EntityID) bool { return r.isAllocType(ent, "Shared", "Box") }

// mergeOp ORs two summaries; the first state found wins as the cause, same
// rule applyOpEdge uses, so drop composition stays deterministic too.
func mergeOp(a, b OpSummary) OpSummary {
	out := OpSummary{Known: true, Unresolved: a.Unresolved || b.Unresolved, Untrusted: a.Untrusted || b.Untrusted, Contracts: unionContracts(a.Contracts, b.Contracts)}
	for f := OpFact(0); f < numOpFacts; f++ {
		switch {
		case a.Facts[f].Maybe && !(a.Facts[f].Contract && b.Facts[f].Maybe && !b.Facts[f].Contract):
			out.Facts[f] = a.Facts[f]
		case b.Facts[f].Maybe:
			out.Facts[f] = b.Facts[f]
		}
	}
	return out
}
