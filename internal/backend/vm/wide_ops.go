package vm

import "math"

func (m *Machine) overflow128(w wide, nk int, over bool) *obj {
	if over {
		m.abort("integer overflow")
	}
	return mkWide(w, nk)
}

// binop128 is binop for 128-bit operands; a shift count is a usize, so it reads b.i.
func (m *Machine) binop128(op string, a, b *obj) *obj {
	x, nk := widen(a), a.nkind()
	sg := isSigned(nk)
	if op == "<<" || op == ">>" {
		if b.i < 0 || b.i >= 128 {
			m.abort("shift count out of range")
		}
		switch {
		case op == "<<":
			r := shlW(x, uint(b.i))
			back := shrW(r, uint(b.i))
			if sg {
				back = sarW(r, uint(b.i))
			}
			return m.overflow128(r, nk, back != x)
		case sg:
			return mkWide(sarW(x, uint(b.i)), nk)
		}
		return mkWide(shrW(x, uint(b.i)), nk)
	}
	y := widen(b)
	switch op {
	case "+":
		r := addW(x, y)
		if sg {
			return m.overflow128(r, nk, x.neg() == y.neg() && r.neg() != x.neg())
		}
		return m.overflow128(r, nk, cmpU(r, x) < 0)
	case "-":
		r := subW(x, y)
		if sg {
			return m.overflow128(r, nk, x.neg() != y.neg() && r.neg() != x.neg())
		}
		return m.overflow128(r, nk, cmpU(x, y) < 0)
	case "*":
		var r wide
		var over bool
		if sg {
			r, over = mulS(x, y)
		} else {
			r, over = mulU(x, y)
		}
		return m.overflow128(r, nk, over)
	case "/", "%":
		if y.zero() {
			m.abort("division by zero")
		}
		var q, r wide
		if sg {
			if y == (wide{^uint64(0), ^uint64(0)}) && x == (wide{0, 1 << 63}) {
				m.abort("integer overflow")
			}
			q, r = divmodS(x, y)
		} else {
			q, r = divmodU(x, y)
		}
		if op == "/" {
			return mkWide(q, nk)
		}
		return mkWide(r, nk)
	case "&":
		return mkWide(wide{x.lo & y.lo, x.hi & y.hi}, nk)
	case "|":
		return mkWide(wide{x.lo | y.lo, x.hi | y.hi}, nk)
	case "^":
		return mkWide(wide{x.lo ^ y.lo, x.hi ^ y.hi}, nk)
	}
	m.abort("unsupported operator " + op)
	return nil
}

func (m *Machine) unop128(op byte, a *obj) *obj {
	x := widen(a)
	switch op {
	case '-':
		return m.overflow128(negW(x), a.nkind(), x == wide{0, 1 << 63})
	case '~':
		return mkWide(notW(x), a.nkind())
	}
	m.abort("unsupported unary " + string(op))
	return nil
}

func cmp128(a, b *obj) int {
	if isSigned(a.nkind()) {
		return cmpS(widen(a), widen(b))
	}
	return cmpU(widen(a), widen(b))
}

// wrapping128 is prelude.<T>.wrapping* / rotate* at 128 bits; the count is masked to the width.
func wrapping128(op string, a0, a1 *obj) *obj {
	x, y, nk := widen(a0), widen(a1), a0.nkind()
	c := uint(y.lo & 127)
	switch op {
	case "wrappingAdd":
		return mkWide(addW(x, y), nk)
	case "wrappingSub":
		return mkWide(subW(x, y), nk)
	case "wrappingMul":
		p, _ := mulU(x, y)
		return mkWide(p, nk)
	case "wrappingShiftLeft":
		return mkWide(shlW(x, c), nk)
	case "wrappingShiftRight":
		if isSigned(nk) {
			return mkWide(sarW(x, c), nk)
		}
		return mkWide(shrW(x, c), nk)
	case "rotateLeft":
		return mkWide(rotlW(x, c), nk)
	case "rotateRight":
		return mkWide(rotlW(x, (128-c)&127), nk)
	}
	return nil
}

// rangeLen128 counts the values of a 128-bit range, saturating at the largest length an iteration can reach.
func rangeLen128(lo, hi *obj, inclusive bool) int64 {
	a, b := widen(lo), widen(hi)
	if cmp128(hi, lo) < 0 {
		return 0
	}
	d := subW(b, a)
	if d.hi != 0 || d.lo >= math.MaxInt64 {
		return math.MaxInt64
	}
	if inclusive {
		return int64(d.lo) + 1
	}
	return int64(d.lo)
}
