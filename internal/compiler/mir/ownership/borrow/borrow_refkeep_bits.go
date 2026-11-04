package borrow

import "slices"

// refBits is a set of indices of any width.
type refBits []uint64

func (b refBits) has(i int) bool { return i >= 0 && i/64 < len(b) && b[i/64]&(1<<(i%64)) != 0 }

func (b refBits) with(i int) refBits {
	out := slices.Clone(b)
	for len(out) <= i/64 {
		out = append(out, 0)
	}
	out[i/64] |= 1 << (i % 64)
	return out
}

func (b refBits) union(o refBits) refBits {
	out := slices.Clone(b)
	for i, w := range o {
		for len(out) <= i {
			out = append(out, 0)
		}
		out[i] |= w
	}
	return out
}

func (b refBits) empty() bool { return !slices.ContainsFunc(b, func(w uint64) bool { return w != 0 }) }

func (b refBits) same(o refBits) bool {
	for i := range max(len(b), len(o)) {
		var x, y uint64
		if i < len(b) {
			x = b[i]
		}
		if i < len(o) {
			y = o[i]
		}
		if x != y {
			return false
		}
	}
	return true
}

// refKind is which borrow of an argument slot a keep names: the borrow the argument is (a reference, or a lent
// receiver's place), or the borrows its value or referent holds.
type refKind int

const (
	refOwn refKind = iota
	refHeld
)

func refBit(j int, kind refKind) int { return 2*j + int(kind) }

// refSummary is what a body may keep of what its argument slots lend, by refBit: in its result, in the place of
// each `&mut` / `mut self` / holder slot, and excl for an own borrow it may keep exclusive; all for one it cannot follow.
type refSummary struct {
	result refBits
	stores map[int]refBits
	excl   refBits
	all    bool
}

func (s *refSummary) keepsResult(j int, kind refKind) bool {
	return s.all || s.result.has(refBit(j, kind))
}

func (s *refSummary) storesInto(t, j int, kind refKind) bool {
	return s.all || s.stores[t].has(refBit(j, kind))
}

func (s *refSummary) keepsExcl(j int) bool { return s.all || s.excl.has(refBit(j, refOwn)) }

func (s *refSummary) keepsAny() bool {
	return s.all || !s.result.empty() || slices.ContainsFunc(sortedStores(s), func(b refBits) bool { return !b.empty() })
}

func (s *refSummary) same(o *refSummary) bool {
	if s.all != o.all || !s.result.same(o.result) || !s.excl.same(o.excl) || len(s.stores) != len(o.stores) {
		return false
	}
	for t, b := range s.stores {
		if !b.same(o.stores[t]) {
			return false
		}
	}
	return true
}

func (s *refSummary) join(o *refSummary) *refSummary {
	out := &refSummary{result: s.result.union(o.result), excl: s.excl.union(o.excl), all: s.all || o.all, stores: map[int]refBits{}}
	for t, b := range s.stores {
		out.stores[t] = out.stores[t].union(b)
	}
	for t, b := range o.stores {
		out.stores[t] = out.stores[t].union(b)
	}
	return out
}

func sortedStores(s *refSummary) []refBits {
	var out []refBits
	for _, b := range s.stores {
		out = append(out, b)
	}
	return out
}

// addWindow adds a keptBody refs run over slots base..base+31: bit i its own kind, bit contentShift+i its held kind.
func (s *refSummary) addWindow(w *keptSummary, base int) {
	if w.all {
		s.all = true
		return
	}
	widen := func(bits uint64) refBits {
		var out refBits
		for i := range contentShift {
			if bits&(1<<i) != 0 {
				out = out.with(refBit(base+i, refOwn))
			}
			if bits&(1<<(i+contentShift)) != 0 {
				out = out.with(refBit(base+i, refHeld))
			}
		}
		return out
	}
	s.result = s.result.union(widen(w.result))
	s.excl = s.excl.union(widen(w.excl))
	if s.stores == nil {
		s.stores = map[int]refBits{}
	}
	for t, bits := range w.stores {
		s.stores[t] = s.stores[t].union(widen(bits))
	}
}
