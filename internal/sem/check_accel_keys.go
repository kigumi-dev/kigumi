package sem

// checkAccelKeys rejects bodiless std overloads sharing a dispatch key (package+owner+name,
// no signature) but differing in signature; a bodied fn is exempt, its accel table being per-engine.
func (r *Result) checkAccelKeys() {
	type slot struct {
		fn  EntityID
		sig string
	}
	groups := map[string][]slot{}
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Kind != EntFn || e.Flags&EfStd == 0 || e.Flags&EfPoison != 0 {
			continue
		}
		info := r.Fn(EntityID(id))
		if info.Abi != "" || info.Body != 0 {
			continue
		}
		key := r.Packages[e.Pkg].Path + "."
		if info.Owner != 0 {
			key += r.Entities[info.Owner].Name + "."
		}
		key += e.Name
		groups[key] = append(groups[key], slot{EntityID(id), r.TypeString(info.Sig)})
	}
	for _, g := range groups {
		for i := 1; i < len(g); i++ {
			if g[i].sig == g[0].sig {
				continue
			}
			e := &r.Entities[g[i].fn]
			r.errNote(e.File, e.Node, Binding{Ent: g[0].fn}, "the colliding overload is here",
				cAccelKeyCollision, e.Name, r.Entities[g[0].fn].Name)
		}
	}
}
