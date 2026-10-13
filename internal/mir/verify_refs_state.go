package mir

import "slices"

// A local's slot holds objects, one per reference the frame took, or a marker:
// never assigned, nulled by a move or a drop, or a view of another's value.
const (
	refUndef  int32 = -1
	refNulled int32 = -2
	refLent   int32 = -3
)

// An object is the allocation site that made it, 3*site, and the state of
// its reference: still held, released, or not pinned down.
const (
	objLive    int32 = 0
	objDead    int32 = 1
	objUnknown int32 = 2
)

func objState(o int32) int32 { return o % 3 }
func objSite(o int32) int32  { return o / 3 }

type refState struct {
	sets *objSets
	ptr  []int32
}

func (s *refState) clone() *refState {
	return &refState{sets: s.sets, ptr: slices.Clone(s.ptr)}
}

func (s *refState) holds(l LocalID) []int32 { return s.sets.sets[s.ptr[l]] }

func (s *refState) join(o *refState) bool {
	changed := false
	for l, p := range s.ptr {
		if p != o.ptr[l] {
			if u := s.sets.union(p, o.ptr[l]); u != p {
				s.ptr[l], changed = u, true
			}
		}
	}
	return changed
}

// gone says a read may see a value with no reference left, on some path. A
// local that may be unassigned is spared: a pattern binding is only set on a match.
func (s *refState) gone(l LocalID) bool {
	dead := false
	for _, o := range s.holds(l) {
		switch {
		case o == refUndef:
			return false
		case o == refNulled || (o >= 0 && objState(o) == objDead):
			dead = true
		}
	}
	return dead
}

func (s *refState) spent(l LocalID) bool {
	for _, o := range s.holds(l) {
		if o == refUndef || o == refLent || (o >= 0 && objState(o) != objDead) {
			return false
		}
	}
	return len(s.holds(l)) > 0
}

func (s *refState) retag(obj, to int32) {
	for l, p := range s.ptr {
		if !s.sets.objects[p] {
			continue
		}
		set := s.sets.sets[p]
		if !slices.Contains(set, obj) {
			continue
		}
		next := make([]int32, len(set))
		for i, o := range set {
			if o == obj {
				o = objSite(obj)*3 + to
			}
			next[i] = o
		}
		s.ptr[l] = s.sets.of(next...)
	}
}

func (s *refState) retagOthers(l LocalID, obj int32) {
	for m, p := range s.ptr {
		set := s.sets.sets[p]
		if LocalID(m) == l || !s.sets.objects[p] || !slices.Contains(set, obj) {
			continue
		}
		to := objUnknown
		if len(set) == 1 {
			to = objDead
		}
		s.ptr[m] = s.sets.of(slices.Replace(slices.Clone(set), slices.Index(set, obj), slices.Index(set, obj)+1, objSite(obj)*3+to)...)
	}
}

func (s *refState) release(l LocalID) bool {
	objs := slices.Clone(s.holds(l))
	if len(objs) == 1 && objs[0] >= 0 {
		switch objState(objs[0]) {
		case objDead:
			return true
		case objLive:
			s.retag(objs[0], objDead)
		}
		return false
	}
	if s.spent(l) || releasedOnSomePath(objs) {
		return true
	}
	after := make([]int32, len(objs))
	for i, o := range objs {
		after[i] = o
		if o >= 0 && objState(o) == objLive {
			after[i] = objSite(o)*3 + objDead
			s.retagOthers(l, o)
		}
	}
	s.ptr[l] = s.sets.of(after...)
	return false
}

func (s *refState) fresh(l LocalID, site int32, loop bool) {
	if loop {
		for m := range s.ptr {
			for _, o := range s.holds(LocalID(m)) {
				if o >= 0 && objSite(o) == site && objState(o) == objLive {
					s.retag(o, objUnknown)
				}
			}
		}
	}
	s.ptr[l] = s.sets.single(site * 3)
}

func releasedOnSomePath(objs []int32) bool {
	for _, o := range objs {
		if o >= 0 && objState(o) != objDead && slices.Contains(objs, o-objState(o)+objDead) {
			return true
		}
	}
	return false
}
