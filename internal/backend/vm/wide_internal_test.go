package vm

import (
	"math/big"
	mathrand "math/rand"
	"testing"
)

var (
	two128 = new(big.Int).Lsh(big.NewInt(1), 128)
	two127 = new(big.Int).Lsh(big.NewInt(1), 127)
)

func (w wide) unsigned() *big.Int { return bigOfWide(w, false) }

func (w wide) signedInt() *big.Int { return bigOfWide(w, true) }

func randWide(r *mathrand.Rand) wide {
	pick := func() uint64 {
		switch r.Intn(6) {
		case 0:
			return 0
		case 1:
			return ^uint64(0)
		case 2:
			return 1 << 63
		case 3:
			return uint64(r.Intn(1000))
		}
		return r.Uint64()
	}
	w := wide{pick(), pick()}
	if r.Intn(4) == 0 {
		w.hi = 0
	}
	return w
}

func TestWideArithmeticAgainstBig(t *testing.T) {
	r := mathrand.New(mathrand.NewSource(7))
	for range 20000 {
		a, b := randWide(r), randWide(r)
		ua, ub := a.unsigned(), b.unsigned()
		sa, sb := a.signedInt(), b.signedInt()
		if got := addW(a, b).unsigned(); got.Cmp(new(big.Int).Mod(new(big.Int).Add(ua, ub), two128)) != 0 {
			t.Fatalf("add %v %v: %v", ua, ub, got)
		}
		if got := subW(a, b).unsigned(); got.Cmp(new(big.Int).Mod(new(big.Int).Sub(ua, ub), two128)) != 0 {
			t.Fatalf("sub %v %v: %v", ua, ub, got)
		}
		p, over := mulU(a, b)
		full := new(big.Int).Mul(ua, ub)
		if p.unsigned().Cmp(new(big.Int).Mod(full, two128)) != 0 || over != (full.Cmp(two128) >= 0) {
			t.Fatalf("mulU %v %v: %v %v", ua, ub, p.unsigned(), over)
		}
		ps, sover := mulS(a, b)
		sfull := new(big.Int).Mul(sa, sb)
		fits := sfull.Cmp(new(big.Int).Neg(two127)) >= 0 && sfull.Cmp(two127) < 0
		if sover == fits || fits && ps.signedInt().Cmp(sfull) != 0 {
			t.Fatalf("mulS %v %v: %v %v", sa, sb, ps.signedInt(), sover)
		}
		if !b.zero() {
			q, rem := divmodU(a, b)
			wq, wr := new(big.Int).QuoRem(ua, ub, new(big.Int))
			if q.unsigned().Cmp(wq) != 0 || rem.unsigned().Cmp(wr) != 0 {
				t.Fatalf("divmodU %v %v: %v %v", ua, ub, q.unsigned(), rem.unsigned())
			}
			if !(sa.Cmp(new(big.Int).Neg(two127)) == 0 && sb.Cmp(big.NewInt(-1)) == 0) {
				q, rem = divmodS(a, b)
				wq, wr = new(big.Int).QuoRem(sa, sb, new(big.Int))
				if q.signedInt().Cmp(wq) != 0 || rem.signedInt().Cmp(wr) != 0 {
					t.Fatalf("divmodS %v %v: %v %v", sa, sb, q.signedInt(), rem.signedInt())
				}
			}
		}
		if got, want := cmpU(a, b), ua.Cmp(ub); got != want {
			t.Fatalf("cmpU %v %v: %d", ua, ub, got)
		}
		if got, want := cmpS(a, b), sa.Cmp(sb); got != want {
			t.Fatalf("cmpS %v %v: %d", sa, sb, got)
		}
		n := uint(r.Intn(128))
		if got := shlW(a, n).unsigned(); got.Cmp(new(big.Int).Mod(new(big.Int).Lsh(ua, n), two128)) != 0 {
			t.Fatalf("shl %v %d: %v", ua, n, got)
		}
		if got := shrW(a, n).unsigned(); got.Cmp(new(big.Int).Rsh(ua, n)) != 0 {
			t.Fatalf("shr %v %d: %v", ua, n, got)
		}
		if got := sarW(a, n).signedInt(); got.Cmp(new(big.Int).Rsh(sa, n)) != 0 {
			t.Fatalf("sar %v %d: %v", sa, n, got)
		}
		if got := wideDecimal(a, false); got != ua.String() {
			t.Fatalf("decimal %v: %s", ua, got)
		}
		if got := wideDecimal(a, true); got != sa.String() {
			t.Fatalf("signed decimal %v: %s", sa, got)
		}
	}
}

func TestWideFloatConversionRoundsLikeHardware(t *testing.T) {
	r := mathrand.New(mathrand.NewSource(11))
	for range 20000 {
		a := randWide(r)
		a = shrW(a, uint(r.Intn(100)))
		want, _ := new(big.Float).SetInt(a.unsigned()).Float64()
		if got := wideToFloat(false, a, 53); got != want {
			t.Fatalf("toFloat %v: %v, want %v", a.unsigned(), got, want)
		}
		want32, _ := new(big.Float).SetInt(a.unsigned()).Float32()
		if got := float32(wideToFloat(false, a, 24)); got != want32 {
			t.Fatalf("toF32 %v: %v, want %v", a.unsigned(), got, want32)
		}
	}
}

func TestWideFromFloatTruncates(t *testing.T) {
	for _, f := range []float64{0, 0.5, -0.5, 1.9, -1.9, 1e19, -1e19, 1.5e30, -1.5e38, 3.4e38, 1e38} {
		neg, mag, ok := floatToWide(f)
		if !ok {
			t.Fatalf("%v rejected", f)
		}
		want, _ := new(big.Float).SetFloat64(f).Int(nil)
		got := mag.unsigned()
		if neg {
			got.Neg(got)
		}
		if got.Cmp(want) != 0 {
			t.Errorf("%v: got %v, want %v", f, got, want)
		}
	}
	for _, f := range []float64{3.5e38, 1e39, -1e39} {
		if _, _, ok := floatToWide(f); ok {
			t.Errorf("%v accepted", f)
		}
	}
}
