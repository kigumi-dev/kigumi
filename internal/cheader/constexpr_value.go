package cheader

import "math/big"

type cval struct {
	v      *big.Int
	bits   int
	signed bool
}

func (c cval) typeName() string {
	prefix := "u"
	if c.signed {
		prefix = "i"
	}
	return prefix + map[int]string{8: "8", 16: "16", 32: "32", 64: "64"}[c.bits]
}

func (c cval) fits(v *big.Int) bool {
	lo, hi := new(big.Int), new(big.Int).Lsh(big.NewInt(1), uint(c.bits))
	if c.signed {
		hi.Rsh(hi, 1)
		lo.Neg(hi)
	}
	return v.Cmp(lo) >= 0 && v.Cmp(hi) < 0
}

func (c cval) wrap(v *big.Int) (cval, bool) {
	if c.signed {
		c.v = v
		return c, c.fits(v)
	}
	mod := new(big.Int).Lsh(big.NewInt(1), uint(c.bits))
	c.v = new(big.Int).Mod(v, mod)
	return c, true
}

func scalarVal(t string, v *big.Int) (cval, bool) {
	c := cval{v: v}
	switch t {
	case "i8", "i16", "i32", "i64", "isize":
		c.signed = true
	case "u8", "u16", "u32", "u64", "usize":
	default:
		return cval{}, false
	}
	c.bits = 64
	if t != "isize" && t != "usize" {
		switch t[1:] {
		case "8":
			c.bits = 8
		case "16":
			c.bits = 16
		case "32":
			c.bits = 32
		}
	}
	return c, c.fits(v)
}

func (c cval) promoted() cval {
	if c.bits < 32 {
		return cval{v: c.v, bits: 32, signed: true}
	}
	return c
}

func arith(a, b cval) cval {
	a, b = a.promoted(), b.promoted()
	switch {
	case a.bits > b.bits:
		return a
	case b.bits > a.bits:
		return b
	case !a.signed:
		return a
	}
	return b
}
