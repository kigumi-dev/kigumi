package sem

// reportEqHash warns when one half of the Eq/Hash pair is written by the
// author and the other is compiler-derived. A derived method compares or
// hashes every field, so it can silently disagree with a custom rule; both
// halves must come from the same decision.
func (r *Result) reportEqHash() {
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Kind != EntType || e.File == 0 || e.Node == 0 || e.Flags&EfPoison != 0 {
			continue
		}
		if r.Packages[e.Pkg].Std {
			continue
		}
		info := r.typeDecl(EntityID(id))
		eqManual, eqDerived := r.protoMemberKind(info, "equals")
		hashManual, hashDerived := r.protoMemberKind(info, "hash")
		var defined, missing string
		switch {
		case eqManual && !hashManual && hashDerived:
			defined, missing = "equals", "hash"
		case hashManual && !eqManual && eqDerived:
			defined, missing = "hash", "equals"
		default:
			continue
		}
		r.errAt(e.File, e.Node, cEqHashMismatch, e.Name, defined, missing)
	}
}

// protoMemberKind reports whether info has a hand-written or a generated
// member of the protocol method's name.
func (r *Result) protoMemberKind(info *TypeDeclInfo, name string) (manual, derived bool) {
	set, ok := info.Members[name]
	if !ok {
		return false, false
	}
	for _, m := range r.Overloads[set].Members {
		if r.ProtoKind(m) == name {
			derived = true
		} else {
			manual = true
		}
	}
	return manual, derived
}
