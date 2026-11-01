package flow

import (
	"math/bits"

	midir "kigumi/internal/compiler/mir/ir"
)

type Bitset []uint64

func (b Bitset) Has(l midir.LocalID) bool { return b[l/64]&(1<<(l%64)) != 0 }
func (b Bitset) Set(l midir.LocalID)      { b[l/64] |= 1 << (l % 64) }
func (b Bitset) Clear(l midir.LocalID)    { b[l/64] &^= 1 << (l % 64) }

func (b Bitset) Each(fn func(l midir.LocalID)) {
	for w, x := range b {
		for ; x != 0; x &= x - 1 {
			fn(midir.LocalID(w*64 + bits.TrailingZeros64(x)))
		}
	}
}
