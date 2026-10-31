package sem

import "sort"

// PackageMembers lists the top-level names of pkg that code in package from may use after `pkg.`, sorted by name.
func (r *Result) PackageMembers(pkg, from PackageID) []Member {
	var out []Member
	for name, b := range r.Scopes[r.Packages[pkg].Scope].Names {
		if name == "" || name[0] == '$' {
			continue
		}
		ent := b.Ent
		if ent == 0 && b.Set != 0 && len(r.Overloads[b.Set].Members) > 0 {
			ent = r.Overloads[b.Set].Members[0]
		}
		if ent == 0 || !r.visibleFrom(r.Entities[ent].Vis, from) || r.isStdExport(ent) {
			continue
		}
		out = append(out, Member{Name: name, Ent: ent})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// isStdExport reports a std runtime override point (export(C)), which is a C symbol rather than package API.
func (r *Result) isStdExport(ent EntityID) bool {
	e := &r.Entities[ent]
	return e.Kind == EntFn && e.Flags&EfStd != 0 && r.Fn(ent).Export != ""
}
