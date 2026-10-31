package sem

import "slices"

// ClosureOrdinal numbers closures of one parent in source order from 1; a copy keeps its origin's number.
func (r *Result) ClosureOrdinal(ent EntityID) int {
	ent = r.ClosureOrigin(ent)
	r.ordinalMu.Lock()
	defer r.ordinalMu.Unlock()
	if r.closureOrdinals == nil {
		r.closureOrdinals = r.numberClosures()
	}
	return r.closureOrdinals[ent]
}

func (r *Result) numberClosures() map[EntityID]int {
	type group struct {
		parent EntityID
		pkg    PackageID
	}
	groups := map[group][]EntityID{}
	for id := range r.Entities {
		e := &r.Entities[id]
		if _, copied := r.closureOrigins[EntityID(id)]; e.Kind != EntClosure || copied || r.closure(EntityID(id)).IsMethodValue() {
			continue
		}
		g := group{e.Parent, e.Pkg}
		groups[g] = append(groups[g], EntityID(id))
	}
	out := make(map[EntityID]int, len(r.Entities)/8)
	for _, ids := range groups {
		slices.SortStableFunc(ids, func(a, b EntityID) int {
			ea, eb := &r.Entities[a], &r.Entities[b]
			if ea.File != eb.File {
				return int(ea.File) - int(eb.File)
			}
			return int(ea.Tok) - int(eb.Tok)
		})
		for i, id := range ids {
			out[id] = i + 1
		}
	}
	return out
}
