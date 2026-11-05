package llgen

import (
	"fmt"
	"math"
	"strings"
)

// A register integer narrower than 64 bits is held sign- or zero-extended to i64.
func intBits(nk int) int { return nk & 255 }

func intTy(nk int) string { return fmt.Sprintf("i%d", intBits(nk)) }

func intMin(nk int) int64 {
	if !isSignedNk(nk) {
		return 0
	}
	if intBits(nk) >= 64 {
		return math.MinInt64
	}
	return -(int64(1) << (intBits(nk) - 1))
}

func (e *emitter) narrowInt(sb *strings.Builder, nk int, v string) string {
	if intBits(nk) >= 64 {
		return v
	}
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = trunc i64 %s to %s\n", t, v, intTy(nk))
	return t
}

func (e *emitter) widenInt(sb *strings.Builder, nk int, v string) string {
	if intBits(nk) >= 64 {
		return v
	}
	ext := "zext"
	if isSignedNk(nk) {
		ext = "sext"
	}
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = %s %s %s to i64\n", t, ext, intTy(nk), v)
	return t
}

func (e *emitter) truncateTo(sb *strings.Builder, nk int, v string) string {
	return e.widenInt(sb, nk, e.narrowInt(sb, nk, v))
}

func (e *emitter) outOfRange(sb *strings.Builder, nk int, v string) string {
	if intBits(nk) >= 64 {
		return "false"
	}
	return e.icmp(sb, "ne", TI64, vi64text(v), vi64text(e.truncateTo(sb, nk, v)))
}

func (e *emitter) checkedArith(sb *strings.Builder, op string, nk int, l, r Value) Value {
	sign := "u"
	if isSignedNk(nk) {
		sign = "s"
	}
	v, o := e.withOverflow(sb, sign+op, intTy(nk), e.narrowInt(sb, nk, l.Text), e.narrowInt(sb, nk, r.Text))
	e.overflowTrap(sb, o, "integer overflow")
	return vi64text(e.widenInt(sb, nk, v))
}

func (e *emitter) checkedDiv(sb *strings.Builder, op string, nk int, l, r Value) Value {
	e.overflowTrap(sb, e.icmp(sb, "eq", TI64, r, vi64(0)), "division by zero")
	signed := isSignedNk(nk)
	if signed {
		m1 := e.icmp(sb, "eq", TI64, r, vi64(-1))
		lo := e.icmp(sb, "eq", TI64, l, vi64text(fmt.Sprint(intMin(nk))))
		e.overflowTrap(sb, e.binOpT(sb, "and", TI1, Value{TI1, m1}, Value{TI1, lo}), "integer overflow")
	}
	kind := map[string][2]string{"/": {"sdiv", "udiv"}, "%": {"srem", "urem"}}[op]
	if signed {
		return vi64text(e.binOp(sb, kind[0], l, r))
	}
	return vi64text(e.binOp(sb, kind[1], l, r))
}

func (e *emitter) checkedShift(sb *strings.Builder, op string, nk int, l, r Value) Value {
	e.overflowTrap(sb, e.icmp(sb, "uge", TI64, r, vi64(intBits(nk))), "shift count out of range")
	if op == ">>" {
		if isSignedNk(nk) {
			return vi64text(e.binOp(sb, "ashr", l, r))
		}
		return vi64text(e.binOp(sb, "lshr", l, r))
	}
	v := e.binOp(sb, "shl", l, r)
	if bad := e.outOfRange(sb, nk, v); bad != "false" {
		e.overflowTrap(sb, bad, "integer overflow")
	}
	return vi64text(v)
}

func (e *emitter) checkedNeg(sb *strings.Builder, nk int, x Value) Value {
	v := e.binOp(sb, "sub", vi64(0), x)
	bad := e.icmp(sb, "eq", TI64, x, vi64text(fmt.Sprint(intMin(nk))))
	if wide := e.outOfRange(sb, nk, v); wide != "false" {
		bad = e.binOpT(sb, "or", TI1, Value{TI1, bad}, Value{TI1, wide})
	}
	e.overflowTrap(sb, bad, "integer overflow")
	return vi64text(v)
}

func (e *emitter) bitNot(sb *strings.Builder, nk int, x Value) Value {
	return vi64text(e.truncateTo(sb, nk, e.binOp(sb, "xor", x, vi64(-1))))
}
