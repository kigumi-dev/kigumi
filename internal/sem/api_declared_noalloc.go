package sem

// DeclaresNoalloc reports whether the module's own code (not std) has a body that promises `noalloc`.
func (r *Result) DeclaresNoalloc() bool {
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Kind != EntFn || e.Flags&(EfStd|EfPoison) != 0 || r.Packages[e.Pkg].Std {
			continue
		}
		if info := r.Fn(EntityID(id)); info.Body != 0 && info.Declared&EffNoalloc != 0 {
			return true
		}
	}
	return false
}
