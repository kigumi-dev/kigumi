package mir

import (
	"math/bits"

	midir "kigumi/internal/compiler/mir/ir"
)

type bitset []uint64

func (b bitset) has(l midir.LocalID) bool { return b[l/64]&(1<<(l%64)) != 0 }
func (b bitset) set(l midir.LocalID)      { b[l/64] |= 1 << (l % 64) }
func (b bitset) clear(l midir.LocalID)    { b[l/64] &^= 1 << (l % 64) }

func (b bitset) each(fn func(l midir.LocalID)) {
	for w, x := range b {
		for ; x != 0; x &= x - 1 {
			fn(midir.LocalID(w*64 + bits.TrailingZeros64(x)))
		}
	}
}
