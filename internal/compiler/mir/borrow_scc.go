package mir

import "slices"

// sccStack holds the open entries of mutually recursive memo computations as Tarjan's strongly connected
// components: a read of an open entry lends its slot to the reader's low, and a component closes at its root.
type sccStack[K comparable] struct {
	stack []K
	slots map[K]int
}

// push opens k and returns its 1-based slot.
func (s *sccStack[K]) push(k K) int {
	if s.slots == nil {
		s.slots = map[K]int{}
	}
	s.stack = append(s.stack, k)
	s.slots[k] = len(s.stack)
	return len(s.stack)
}

func (s *sccStack[K]) slot(k K) (int, bool) {
	v, ok := s.slots[k]
	return v, ok
}

func (s *sccStack[K]) members(slot int) []K { return s.stack[slot-1:] }

// pop closes the component open from slot up and returns its members.
func (s *sccStack[K]) pop(slot int) []K {
	out := slices.Clone(s.stack[slot-1:])
	for _, k := range out {
		delete(s.slots, k)
	}
	s.stack = s.stack[:slot-1]
	return out
}
