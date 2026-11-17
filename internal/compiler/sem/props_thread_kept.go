package sem

import "slices"

// Unlike reachesClosure every capture counts: a closure read in place lives as long as its reader.
func (r *Result) keptClosures(t TypeID) []EntityID {
	if t == 0 || !r.Types.mentionsClosure(t) {
		return nil
	}
	st := &r.thread
	if out, ok := st.kept[t]; ok {
		return out
	}
	var out []EntityID
	add := func(cls []EntityID) {
		for _, cl := range cls {
			if !slices.Contains(out, cl) {
				out = append(out, cl)
			}
		}
	}
	n := r.Types.Node(t)
	if n.Kind == KClosure {
		out = append(out, n.Ent)
		for _, cp := range r.closure(n.Ent).Captures {
			add(r.keptClosures(r.Entities[cp.Local].Type))
		}
	} else {
		add(r.keptClosures(n.Elem))
		for _, a := range n.Args {
			add(r.keptClosures(a))
		}
	}
	if st.settled {
		if st.kept == nil {
			st.kept = map[TypeID][]EntityID{}
		}
		st.kept[t] = out
	}
	return out
}

func (r *Result) keepersOf(cl EntityID) []EntityID {
	st := &r.thread
	if st.keepers == nil {
		st.keepers = map[EntityID][]EntityID{}
		for id := 1; id < len(r.Entities); id++ {
			if k := r.Entities[id].Kind; k != EntLocal && k != EntParam {
				continue
			}
			for _, held := range r.keptClosures(r.Entities[id].Type) {
				st.keepers[held] = append(st.keepers[held], EntityID(id))
			}
		}
	}
	return st.keepers[cl]
}

// A Future keeps its call's arguments without naming them in its type.
func (r *Result) hidesValues(t TypeID) bool {
	if t == 0 {
		return false
	}
	n := r.Types.Node(t)
	switch n.Kind {
	case KIface, KPtr:
		return true
	case KNamed:
		if n.Ent == r.Types.futureEnt || n.Ent == r.taskEntity() {
			return true
		}
	}
	return r.hidesValues(n.Elem) || slices.ContainsFunc(n.Args, r.hidesValues)
}

// A type parameter or a hidden value may be a function value too.
func (r *Result) carriesFn(t TypeID) bool {
	if t == 0 {
		return false
	}
	n := r.Types.Node(t)
	switch {
	case n.Kind == KFn || n.Kind == KParam || r.hidesValues(t):
		return true
	case n.Kind == KClosure:
		return slices.ContainsFunc(r.closure(n.Ent).Captures, func(cp Capture) bool {
			return r.carriesFn(r.Entities[cp.Local].Type)
		})
	}
	return r.carriesFn(n.Elem) || slices.ContainsFunc(n.Args, r.carriesFn)
}
