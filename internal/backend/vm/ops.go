package vm

import (
	"math"
	"math/bits"
)

// checked aborts on overflow of the destination width, like the runtime.
func (m *Machine) checked(v int64, nk int, over bool) *obj {
	if !over {
		if isSigned(nk) {
			over = v < minOf(nk) || v > int64(maxOf(nk))
		} else if bitsOf(nk) < 64 {
			over = v < 0 || uint64(v) > maxOf(nk)
		}
	}
	if over {
		m.abort("integer overflow")
	}
	return m.newInt(v, nk)
}

func (m *Machine) binop(op string, a, b *obj) *obj { return m.binopAs(bopOf(op), op, a, b) }

// binopAs is binopCore with the SecretSan label in place: the operation's
// value-dependent decisions are checked against the operands' labels, and the
// result carries what the labels say it should.
func (m *Machine) binopAs(c bop, op string, a, b *obj) *obj {
	if !m.opts.SecretSan {
		return m.binopCore(c, op, a, b)
	}
	m.checkBinopSinks(c, a, b)
	return m.withTaint(m.binopCore(c, op, a, b), m.binopShadow(c, op, a, b))
}

func (m *Machine) binopCore(c bop, op string, a, b *obj) *obj {
	a, b = deref(a), deref(b)
	switch c {
	case bopEq:
		return mkBool(m.equal(a, b))
	case bopNe:
		return mkBool(!m.equal(a, b))
	case bopOther:
		switch op {
		case "..":
			return mkRecord(m.lookup("std/prelude", "Range"), []*obj{retain(a), retain(b), mkBool(false)})
		case "..=":
			return mkRecord(m.lookup("std/prelude", "Range"), []*obj{retain(a), retain(b), mkBool(true)})
		}
	}
	if a.k == kFloat {
		x, y := a.fl(), b.fl()
		switch c {
		case bopLt:
			return mkBool(x < y)
		case bopLe:
			return mkBool(x <= y)
		case bopGt:
			return mkBool(x > y)
		case bopGe:
			return mkBool(x >= y)
		case bopAdd:
			return m.newFloat(x+y, a.nkind())
		case bopSub:
			return m.newFloat(x-y, a.nkind())
		case bopMul:
			return m.newFloat(x*y, a.nkind())
		case bopDiv:
			return m.newFloat(x/y, a.nkind())
		}
	}
	switch c {
	case bopLt:
		return mkBool(compare(a, b) < 0)
	case bopLe:
		return mkBool(compare(a, b) <= 0)
	case bopGt:
		return mkBool(compare(a, b) > 0)
	case bopGe:
		return mkBool(compare(a, b) >= 0)
	}
	if a.k != kInt {
		m.abort("unsupported operands")
	}
	if is128(a.nkind()) {
		return m.binop128(op, a, b)
	}
	x, y, nk := a.i, b.i, a.nkind()
	sg := isSigned(nk)
	switch c {
	case bopAdd:
		r := x + y
		over := sg && ((x > 0 && y > 0 && r < 0) || (x < 0 && y < 0 && r >= 0)) || !sg && bitsOf(nk) == 64 && uint64(r) < uint64(x)
		return m.checked(r, nk, over)
	case bopSub:
		r := x - y
		over := sg && ((x >= 0 && y < 0 && r < 0) || (x < 0 && y > 0 && r >= 0)) || !sg && uint64(x) < uint64(y)
		return m.checked(r, nk, over)
	case bopMul:
		if sg {
			_, lo := bits.Mul64(uint64(x), uint64(y))
			r := int64(lo)
			over := (x != 0 && r/x != y) || (x == -1 && y == math.MinInt64) || (y == -1 && x == math.MinInt64)
			return m.checked(r, nk, over)
		}
		hi, lo := bits.Mul64(uint64(x), uint64(y))
		return m.checked(int64(lo), nk, hi != 0)
	case bopDiv, bopRem:
		if y == 0 {
			m.abort("division by zero")
		}
		if sg && y == -1 && x == minOf(nk) {
			m.abort("integer overflow")
		}
		if sg {
			if c == bopDiv {
				return m.newInt(x/y, nk)
			}
			return m.newInt(x%y, nk)
		}
		if c == bopDiv {
			return m.newInt(int64(uint64(x)/uint64(y)), nk)
		}
		return m.newInt(int64(uint64(x)%uint64(y)), nk)
	case bopAnd:
		return m.newInt(x&y, nk)
	case bopOr:
		return m.newInt(x|y, nk)
	case bopXor:
		return m.checked(x^y, nk, false)
	case bopShl:
		if y < 0 || y >= int64(bitsOf(nk)) {
			m.abort("shift count out of range")
		}
		return m.checked(x<<uint(y), nk, false)
	case bopShr:
		if y < 0 || y >= int64(bitsOf(nk)) {
			m.abort("shift count out of range")
		}
		if sg {
			return m.newInt(x>>uint(y), nk)
		}
		return m.newInt(int64(uint64(x)>>uint(y)), nk)
	}
	m.abort("unsupported operator " + op)
	return nil
}
