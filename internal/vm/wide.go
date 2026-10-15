package vm

import "math/bits"

// wide is a 128-bit two's complement value; a 128-bit obj keeps it in i (low) and hi.
type wide struct{ lo, hi uint64 }

func is128(nk int) bool { return nk&255 == 128 }

// widen reads an integer of any width as a 128-bit value, extending by the source's sign.
func widen(o *obj) wide {
	if is128(o.nkind()) {
		return wide{uint64(o.i), uint64(o.hiWord())}
	}
	if isSigned(o.nkind()) && o.i < 0 {
		return wide{uint64(o.i), ^uint64(0)}
	}
	return wide{lo: uint64(o.i)}
}

func mkWide(w wide, nk int) *obj {
	v := mkInt(int64(w.lo), nk)
	v.setHi(int64(w.hi))
	return v
}

func (w wide) neg() bool { return w.hi>>63 != 0 }

func (w wide) zero() bool { return w.lo|w.hi == 0 }

func notW(w wide) wide { return wide{^w.lo, ^w.hi} }

func negW(w wide) wide { return addW(notW(w), wide{lo: 1}) }

func absW(w wide) wide {
	if w.neg() {
		return negW(w)
	}
	return w
}

func addW(a, b wide) wide {
	lo, c := bits.Add64(a.lo, b.lo, 0)
	hi, _ := bits.Add64(a.hi, b.hi, c)
	return wide{lo, hi}
}

func subW(a, b wide) wide {
	lo, c := bits.Sub64(a.lo, b.lo, 0)
	hi, _ := bits.Sub64(a.hi, b.hi, c)
	return wide{lo, hi}
}

func cmpU(a, b wide) int {
	if a.hi != b.hi {
		return cmp3(a.hi < b.hi, a.hi > b.hi)
	}
	return cmp3(a.lo < b.lo, a.lo > b.lo)
}

func cmpS(a, b wide) int {
	if a.hi != b.hi {
		return cmp3(int64(a.hi) < int64(b.hi), int64(a.hi) > int64(b.hi))
	}
	return cmp3(a.lo < b.lo, a.lo > b.lo)
}

func shlW(a wide, n uint) wide {
	switch {
	case n == 0:
		return a
	case n >= 64:
		return wide{0, a.lo << (n - 64)}
	}
	return wide{a.lo << n, a.hi<<n | a.lo>>(64-n)}
}

func shrW(a wide, n uint) wide {
	switch {
	case n == 0:
		return a
	case n >= 64:
		return wide{a.hi >> (n - 64), 0}
	}
	return wide{a.lo>>n | a.hi<<(64-n), a.hi >> n}
}

func sarW(a wide, n uint) wide {
	switch {
	case n == 0:
		return a
	case n >= 64:
		return wide{uint64(int64(a.hi) >> (n - 64)), uint64(int64(a.hi) >> 63)}
	}
	return wide{a.lo>>n | a.hi<<(64-n), uint64(int64(a.hi) >> n)}
}

func rotlW(a wide, n uint) wide {
	if n == 0 {
		return a
	}
	l, r := shlW(a, n), shrW(a, 128-n)
	return wide{l.lo | r.lo, l.hi | r.hi}
}

func mulU(a, b wide) (wide, bool) {
	h1, lo := bits.Mul64(a.lo, b.lo)
	h2, l2 := bits.Mul64(a.hi, b.lo)
	h3, l3 := bits.Mul64(a.lo, b.hi)
	hi, c1 := bits.Add64(h1, l2, 0)
	hi, c2 := bits.Add64(hi, l3, 0)
	over := a.hi != 0 && b.hi != 0 || h2 != 0 || h3 != 0 || c1 != 0 || c2 != 0
	return wide{lo, hi}, over
}

func mulS(a, b wide) (wide, bool) {
	neg := a.neg() != b.neg()
	p, over := mulU(absW(a), absW(b))
	if over {
		return p, true
	}
	if neg {
		return negW(p), p.hi > 1<<63 || p.hi == 1<<63 && p.lo != 0
	}
	return p, p.hi >= 1<<63
}

func divU64(a wide, v uint64) (wide, uint64) {
	var q wide
	var r uint64
	if a.hi < v {
		q.lo, r = bits.Div64(a.hi, a.lo, v)
		return q, r
	}
	q.hi, r = bits.Div64(0, a.hi, v)
	q.lo, r = bits.Div64(r, a.lo, v)
	return q, r
}

func divmodU(a, b wide) (wide, wide) {
	if b.hi == 0 {
		q, r := divU64(a, b.lo)
		return q, wide{lo: r}
	}
	n := uint(bits.LeadingZeros64(b.hi))
	v1 := shlW(b, n)
	u1 := shrW(a, 1)
	tq, _ := bits.Div64(u1.hi, u1.lo, v1.hi)
	tq >>= 63 - n
	if tq != 0 {
		tq--
	}
	p, _ := mulU(b, wide{lo: tq})
	r := subW(a, p)
	if cmpU(r, b) >= 0 {
		tq++
		r = subW(r, b)
	}
	return wide{lo: tq}, r
}

func divmodS(a, b wide) (wide, wide) {
	q, r := divmodU(absW(a), absW(b))
	if a.neg() != b.neg() {
		q = negW(q)
	}
	if a.neg() {
		r = negW(r)
	}
	return q, r
}
