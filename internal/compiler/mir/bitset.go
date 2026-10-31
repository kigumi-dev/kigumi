package mir

import "math/bits"

type bitset []uint64

func (b bitset) has(l LocalID) bool { return b[l/64]&(1<<(l%64)) != 0 }
func (b bitset) set(l LocalID)      { b[l/64] |= 1 << (l % 64) }
func (b bitset) clear(l LocalID)    { b[l/64] &^= 1 << (l % 64) }

func (b bitset) each(fn func(l LocalID)) {
	for w, x := range b {
		for ; x != 0; x &= x - 1 {
			fn(LocalID(w*64 + bits.TrailingZeros64(x)))
		}
	}
}
