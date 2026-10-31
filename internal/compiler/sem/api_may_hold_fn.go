package sem

func (r *Result) MayHoldFn(t TypeID) bool { return r.mayContainFn(t, map[TypeID]bool{}) }
