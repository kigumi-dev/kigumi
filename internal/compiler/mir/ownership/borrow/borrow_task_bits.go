package borrow

import "math/bits"

// srcSet is a set of Task-handle sources; every operation returns a new set or an unchanged operand, so states can
// share them.
type srcSet []uint64

func (s srcSet) has(i int) bool { return i/64 < len(s) && s[i/64]&(1<<(i%64)) != 0 }

func (s srcSet) with(i int) srcSet {
	if s.has(i) {
		return s
	}
	out := make(srcSet, max(len(s), i/64+1))
	copy(out, s)
	out[i/64] |= 1 << (i % 64)
	return out
}

func (s srcSet) or(o srcSet) srcSet {
	if o.subset(s) {
		return s
	}
	if s.subset(o) {
		return o
	}
	out := make(srcSet, max(len(s), len(o)))
	copy(out, s)
	for i, w := range o {
		out[i] |= w
	}
	return out
}

func (s srcSet) and(o srcSet) srcSet {
	if s.subset(o) {
		return s
	}
	out := make(srcSet, min(len(s), len(o)))
	for i := range out {
		out[i] = s[i] & o[i]
	}
	return out
}

func (s srcSet) minus(o srcSet) srcSet {
	if !s.meets(o) {
		return s
	}
	out := make(srcSet, len(s))
	for i, w := range s {
		if i < len(o) {
			w &^= o[i]
		}
		out[i] = w
	}
	return out
}

func (s srcSet) subset(o srcSet) bool {
	for i, w := range s {
		if i < len(o) {
			w &^= o[i]
		}
		if w != 0 {
			return false
		}
	}
	return true
}

func (s srcSet) meets(o srcSet) bool {
	for i := range min(len(s), len(o)) {
		if s[i]&o[i] != 0 {
			return true
		}
	}
	return false
}

func (s srcSet) anyFrom(n int) bool {
	for i, w := range s {
		if i < n/64 {
			continue
		}
		if i == n/64 {
			w &^= 1<<(n%64) - 1
		}
		if w != 0 {
			return true
		}
	}
	return false
}

func (s srcSet) empty() bool {
	for _, w := range s {
		if w != 0 {
			return false
		}
	}
	return true
}

func (s srcSet) equal(o srcSet) bool { return s.subset(o) && o.subset(s) }

func (s srcSet) each(f func(int)) {
	for i, w := range s {
		for w != 0 {
			b := bits.TrailingZeros64(w)
			f(i*64 + b)
			w &^= 1 << b
		}
	}
}
