package sem

import "math/big"

type shiftFault uint8

const (
	shiftExact shiftFault = iota
	shiftCountRange
	shiftPastWidth
	shiftTooLarge
)

// exactShift is the shift rule both const evaluation and literal folding use; width 0 is an untyped receiver.
func (r *Result) exactShift(x, count *big.Int, width int, left bool) (*big.Int, shiftFault) {
	switch {
	case !r.intFits(count, TyUsize):
		return nil, shiftCountRange
	case width != 0 && count.Uint64() >= uint64(width):
		return nil, shiftPastWidth
	}
	n := count.Uint64()
	z := new(big.Int)
	switch {
	case x.Sign() == 0:
	case !left && n >= uint64(x.BitLen()):
		if x.Sign() < 0 {
			z.SetInt64(-1)
		}
	case !left:
		z.Rsh(x, uint(n))
	case n > maxConstBits || uint64(x.BitLen())+n > maxConstBits:
		return nil, shiftTooLarge
	default:
		z.Lsh(x, uint(n))
	}
	return z, shiftExact
}

const maxConstBits = 1 << 16

// constTooWide is the one size bound for every constant integer result, inclusive of maxConstBits.
func constTooWide(z *big.Int) bool { return z.BitLen() > maxConstBits }

// constProductTooWide refuses a product before it allocates; a product of nonzero values has at least a+b-1 bits.
func constProductTooWide(x, y *big.Int) bool {
	return x.Sign() != 0 && y.Sign() != 0 && x.BitLen()+y.BitLen()-1 > maxConstBits
}
