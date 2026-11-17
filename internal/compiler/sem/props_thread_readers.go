package sem

import "slices"

// A reader that dies inside h does not race the one h returns.
func (r *Result) soleReader(rs ReadSite, h EntityID) bool {
	st := &r.thread
	if !st.settled {
		return false
	}
	if st.readers == nil {
		st.readers = map[EntityID][]EntityID{}
		for id := 1; id < len(r.Entities); id++ {
			if r.Entities[id].Kind != EntClosure {
				continue
			}
			for _, cp := range r.closure(EntityID(id)).Captures {
				if roleOf(cp) == roleFrameRead {
					st.readers[cp.Local] = append(st.readers[cp.Local], EntityID(id))
				}
			}
		}
	}
	return !slices.ContainsFunc(st.readers[rs.Local], func(o EntityID) bool {
		return o != rs.Closure && r.outlives(o, h)
	})
}

func (r *Result) outlives(o, h EntityID) bool {
	if !r.within(o, h) || slices.Contains(r.keptClosures(r.closure(h).Ret), o) {
		return true
	}
	if slices.ContainsFunc(r.keepersOf(o), func(l EntityID) bool { return !r.within(r.frameOf(l), h) }) {
		return true
	}
	return slices.ContainsFunc(r.closureUses(o), func(u closureUse) bool { return u.hides })
}

// An escaped function value can run inside h only through what h captures or takes.
func (r *Result) reachesFnValue(h EntityID) bool {
	info := r.closure(h)
	for _, cp := range info.Captures {
		if r.carriesFn(r.Entities[cp.Local].Type) {
			return true
		}
	}
	return slices.ContainsFunc(info.Params, func(p EntityID) bool { return r.carriesFn(r.Entities[p].Type) })
}
