package vm

import "math"

func bitsOf(nk int) int    { return nk & 255 }
func isSigned(nk int) bool { return nk>>8&1 != 0 }

func minOf(nk int) int64 {
	if !isSigned(nk) {
		return 0
	}
	if b := bitsOf(nk); b < 64 {
		return -(int64(1) << (b - 1))
	}
	return math.MinInt64
}

func maxOf(nk int) uint64 {
	b := bitsOf(nk)
	if isSigned(nk) {
		if b >= 64 {
			return math.MaxInt64
		}
		return uint64(1)<<(b-1) - 1
	}
	if b >= 64 {
		return math.MaxUint64
	}
	return uint64(1)<<b - 1
}

func truncateTo(v int64, nk int) int64 {
	b := bitsOf(nk)
	if b >= 64 {
		return v
	}
	mask := int64(1)<<b - 1
	v &= mask
	if isSigned(nk) && v&(int64(1)<<(b-1)) != 0 {
		v -= int64(1) << b
	}
	return v
}
