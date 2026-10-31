package sem

// The edge keeps fn in Param and the declared type in From for takenUnboxed.
func (r *Result) genericSlot(fn EntityID, declared TypeID) *EffectEdge {
	if fn == 0 || r.Entities[fn].Kind != EntFn || !r.Specializable(fn) {
		return nil
	}
	for _, p := range r.Types.Params(declared) {
		for _, tp := range r.Fn(fn).TypeParams {
			if p == tp {
				return &EffectEdge{Param: fn, From: declared}
			}
		}
	}
	return nil
}

func (r *Result) takenUnboxed(e EffectEdge) bool {
	if e.Why != whyGenericCross {
		return false
	}
	want := map[EntityID]bool{}
	for _, tp := range r.Fn(e.Param).TypeParams {
		want[tp] = !r.IsLifetimeParam(tp)
	}
	bound := map[EntityID]TypeID{}
	if !r.Types.BindParams(e.From, e.Type, want, bound) {
		return false
	}
	for _, t := range bound {
		if r.DirectAgg(t) {
			return true
		}
	}
	return false
}

func (r *Result) DefersAllocClaims() bool { return r.deferredClaims }
