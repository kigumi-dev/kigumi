package sem

// closureForwardSubst passes subst straight through: a capture is the same
// entity inside the closure as outside, unless reassigned (capCell).
func (r *Result) closureForwardSubst(target EntityID, subst map[EntityID][]EntityID) (map[EntityID][]EntityID, bool) {
	if subst == nil || r.Entities[target].Kind != EntClosure {
		return nil, false
	}
	for _, cp := range r.closure(target).Captures {
		if cp.Mode == capCell {
			return nil, false
		}
	}
	return subst, true
}
