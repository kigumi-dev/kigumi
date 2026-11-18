package instance

import (
	"encoding/binary"
	"math/big"
)

// ConstWords is a const argument as the low and high words of its 128-bit two's complement: every const
// parameter type fits, and the runtime keys a const by both words.
func ConstWords(v *big.Int) (lo, hi uint64) {
	m := new(big.Int).Lsh(big.NewInt(1), 128)
	u := new(big.Int).Mod(v, m)
	words := u.FillBytes(make([]byte, 16))
	return binary.BigEndian.Uint64(words[8:]), binary.BigEndian.Uint64(words[:8])
}
