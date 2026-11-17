package sem

func (r *Result) KnownCallees(local EntityID) []EntityID {
	if local == 0 || int(local) >= len(r.Entities) || r.Entities[local].Flags&EfMut != 0 {
		return nil
	}
	var out []EntityID
	for _, e := range r.knownFn[local] {
		if r.Entities[e].Kind == EntFn {
			out = append(out, e)
		}
	}
	return out
}
