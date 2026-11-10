package borrow

import (
	"maps"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

type taskState struct {
	reached bool
	present map[midir.LocalID]srcSet
	joined  srcSet
}

func (s taskState) clone() taskState {
	return taskState{reached: s.reached, present: maps.Clone(s.present), joined: s.joined}
}

func (s *taskState) merge(o taskState) bool {
	if !s.reached {
		*s = o.clone()
		return true
	}
	changed := false
	if j := s.joined.and(o.joined); !j.equal(s.joined) {
		s.joined, changed = j, true
	}
	for l, v := range o.present {
		if u := s.present[l].or(v); !u.equal(s.present[l]) {
			s.present[l], changed = u, true
		}
	}
	return changed
}

const (
	localSeen uint8 = 1 << iota
	localRef
	localHandle
	localBorrow
)

// local classifies l once per run: a reference; a handle holder, which counts a callable since calling it may hand back
// a Task it starts; a possible borrow holder, which also counts a type naming a lifetime or hiding what it holds.
func (r *taskRun) local(l midir.LocalID) uint8 {
	if k := r.kinds[l]; k != 0 {
		return k
	}
	t, k := r.g.Locals[l].Type, localSeen
	if r.c.p.R.Types.Kind(t) == sem.KRef {
		k |= localRef
	}
	if r.g.Locals[l].Cell || r.c.typeRuns(t, runsHandle) || r.c.typeRuns(t, runsCallable) {
		k |= localHandle
	}
	if k&(localRef|localHandle) != 0 || r.c.facts().HoldsBorrow(t) {
		k |= localBorrow
	}
	r.kinds[l] = k
	return k
}

func (r *taskRun) isRef(l midir.LocalID) bool       { return r.local(l)&localRef != 0 }
func (r *taskRun) holdsHandle(l midir.LocalID) bool { return r.local(l)&localHandle != 0 }
func (r *taskRun) mayBorrow(l midir.LocalID) bool   { return r.local(l)&localBorrow != 0 }

// roots is placeRoots of l, kept for the run.
func (r *taskRun) roots(l midir.LocalID) ([]midir.LocalID, bool) {
	if p, ok := r.places[l]; ok {
		return p.roots, p.ok
	}
	roots, ok := r.c.placeRoots(r.g, r.defs, l)
	r.places[l] = placeOf{roots, ok}
	return roots, ok
}

func (r *taskRun) fit(l midir.LocalID, v srcSet) srcSet {
	if !r.holdsHandle(l) {
		v = v.and(r.borrowBits())
	}
	if !r.mayBorrow(l) {
		v = v.minus(r.borrowBits())
	}
	return v
}

func (r *taskRun) assign(st *taskState, l midir.LocalID, v srcSet) {
	if v = r.fit(l, v); v.empty() {
		delete(st.present, l)
		return
	}
	st.present[l] = v
}

func (r *taskRun) add(st *taskState, l midir.LocalID, v srcSet) {
	if v = r.fit(l, v); !v.empty() {
		st.present[l] = st.present[l].or(v)
	}
}
