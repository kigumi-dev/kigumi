package verify

import (
	"math/bits"
	"slices"
)

type wordBit struct {
	w uint32
	b uint64
}

// wordSet is a set of small non-negative ints kept as the nonzero 64-bit words of a bitmap, sorted by word; it stays small while the members cluster.
type wordSet []wordBit

func wordSetOf(ids []int32) wordSet {
	sorted := slices.Clone(ids)
	slices.Sort(sorted)
	var out wordSet
	for _, id := range sorted {
		w, bit := uint32(id)>>6, uint64(1)<<(uint32(id)&63)
		if n := len(out); n > 0 && out[n-1].w == w {
			out[n-1].b |= bit
		} else {
			out = append(out, wordBit{w, bit})
		}
	}
	return out
}

func (s wordSet) has(id int32) bool {
	w := uint32(id) >> 6
	i, ok := slices.BinarySearchFunc(s, w, func(e wordBit, w uint32) int { return int(e.w) - int(w) })
	return ok && s[i].b&(1<<(uint32(id)&63)) != 0
}

// merge walks both sets word by word; keep decides each word from the two sides (zero for an absent word).
func (s wordSet) merge(o wordSet, keep func(a, b uint64) uint64) wordSet {
	out := make(wordSet, 0, max(len(s), len(o)))
	i, j := 0, 0
	for i < len(s) || j < len(o) {
		var w uint32
		var a, b uint64
		switch {
		case j == len(o) || (i < len(s) && s[i].w < o[j].w):
			w, a = s[i].w, s[i].b
			i++
		case i == len(s) || o[j].w < s[i].w:
			w, b = o[j].w, o[j].b
			j++
		default:
			w, a, b = s[i].w, s[i].b, o[j].b
			i, j = i+1, j+1
		}
		if x := keep(a, b); x != 0 {
			out = append(out, wordBit{w, x})
		}
	}
	return out
}

func (s wordSet) union(o wordSet) wordSet {
	if len(o) == 0 {
		return s
	}
	if len(s) == 0 {
		return o
	}
	return s.merge(o, func(a, b uint64) uint64 { return a | b })
}

func (s wordSet) inter(o wordSet) wordSet {
	return s.merge(o, func(a, b uint64) uint64 { return a & b })
}

func (s wordSet) minus(o wordSet) wordSet {
	if len(o) == 0 {
		return s
	}
	return s.merge(o, func(a, b uint64) uint64 { return a &^ b })
}

func (s wordSet) equal(o wordSet) bool { return slices.Equal(s, o) }

func (s wordSet) each(fn func(id int32)) {
	for _, e := range s {
		for x := e.b; x != 0; x &= x - 1 {
			fn(int32(e.w)<<6 + int32(bits.TrailingZeros64(x)))
		}
	}
}
