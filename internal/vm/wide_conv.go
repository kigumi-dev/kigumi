package vm

import (
	"math"
	"math/big"
	"math/bits"
	"strconv"
)

const pow10x19 = 10000000000000000000

func wideDecimal(w wide, signed bool) string {
	if signed && w.neg() {
		return "-" + wideDecimal(negW(w), false)
	}
	if w.hi == 0 {
		return strconv.FormatUint(w.lo, 10)
	}
	var parts [3]uint64
	n := 0
	for !w.zero() {
		var r uint64
		w, r = divU64(w, pow10x19)
		parts[n] = r
		n++
	}
	s := strconv.FormatUint(parts[n-1], 10)
	for i := n - 2; i >= 0; i-- {
		d := strconv.FormatUint(parts[i], 10)
		s += "0000000000000000000"[:19-len(d)] + d
	}
	return s
}

func signMag(o *obj) (bool, wide) {
	w := widen(o)
	if isSigned(o.nk) && w.neg() {
		return true, negW(w)
	}
	return false, w
}

func bitLen(w wide) uint {
	if w.hi != 0 {
		return uint(128 - bits.LeadingZeros64(w.hi))
	}
	return uint(64 - bits.LeadingZeros64(w.lo))
}

// wideToFloat rounds the magnitude to mant bits, ties to even, as the hardware converts.
func wideToFloat(neg bool, mag wide, mant uint) float64 {
	n := bitLen(mag)
	var f float64
	if n <= mant {
		f = float64(mag.lo)
	} else {
		shift := n - mant
		top := shrW(mag, shift).lo
		rem := subW(mag, shlW(wide{lo: top}, shift))
		half := shlW(wide{lo: 1}, shift-1)
		if c := cmpU(rem, half); c > 0 || c == 0 && top&1 == 1 {
			top++
		}
		f = math.Ldexp(float64(top), int(shift))
	}
	if neg {
		return -f
	}
	return f
}

// floatToWide truncates x; ok is false for NaN, an infinity or a magnitude of 2^128 and up.
func floatToWide(x float64) (neg bool, mag wide, ok bool) {
	x = math.Trunc(x)
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return false, wide{}, false
	}
	neg = x < 0
	x = math.Abs(x)
	if x == 0 {
		return false, wide{}, true
	}
	frac, exp := math.Frexp(x)
	if exp > 128 {
		return false, wide{}, false
	}
	m := uint64(frac * (1 << 53))
	if exp >= 53 {
		return neg, shlW(wide{lo: m}, uint(exp-53)), true
	}
	return neg, wide{lo: m >> uint(53-exp)}, true
}

func fitsWide(neg bool, mag wide, dnk int) bool {
	b := uint(bitsOf(dnk))
	one := wide{lo: 1}
	switch {
	case isSigned(dnk):
		limit := shlW(one, b-1)
		if neg {
			return cmpU(mag, limit) <= 0
		}
		return cmpU(mag, limit) < 0
	case neg:
		return mag.zero()
	case b == 128:
		return true
	}
	return cmpU(mag, shlW(one, b)) < 0
}

func fromSignMag(neg bool, mag wide, dnk int) *obj {
	if neg {
		mag = negW(mag)
	}
	if is128(dnk) {
		return mkWide(mag, dnk)
	}
	return mkInt(truncateTo(int64(mag.lo), dnk), dnk)
}

// convertWide is convert when the source or the target is a 128-bit integer.
func (m *Machine) convertWide(a0 *obj, dnk int, checked bool) *obj {
	switch {
	case dnk&512 != 0:
		mant := uint(53)
		if dnk&255 == 32 {
			mant = 24
		}
		neg, mag := signMag(a0)
		return mkFloat(wideToFloat(neg, mag, mant), dnk)
	case a0.k == kFloat:
		neg, mag, ok := floatToWide(a0.f)
		if !ok || !fitsWide(neg, mag, dnk) {
			return m.none()
		}
		return m.some(fromSignMag(neg, mag, dnk))
	}
	neg, mag := signMag(a0)
	if checked && !fitsWide(neg, mag, dnk) {
		return m.none()
	}
	v := fromSignMag(neg, mag, dnk)
	if checked {
		return m.some(v)
	}
	return v
}

// wideOfBig reads x modulo 2^128, so a negative literal comes out in two's complement.
func wideOfBig(x *big.Int) wide {
	m := new(big.Int).And(x, mask128)
	return wide{new(big.Int).And(m, mask64).Uint64(), new(big.Int).Rsh(m, 64).Uint64()}
}

var (
	mask64  = new(big.Int).SetUint64(math.MaxUint64)
	mask128 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
)

// bigOfWide is the value w holds, read as signed or unsigned.
func bigOfWide(w wide, signed bool) *big.Int {
	v := new(big.Int).Or(new(big.Int).Lsh(new(big.Int).SetUint64(w.hi), 64), new(big.Int).SetUint64(w.lo))
	if signed && w.neg() {
		v.Sub(v, new(big.Int).Lsh(big.NewInt(1), 128))
	}
	return v
}
