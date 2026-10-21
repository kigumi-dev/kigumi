package vm

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// std runs a bodiless std function by its runtime key; arguments are lent
// and the result is owned, as with rt_std.
func (m *Machine) std(key string, a []*obj) *obj {
	arg := func(i int) *obj {
		if i < len(a) {
			return deref(a[i])
		}
		return nil
	}
	a0, a1, a2 := arg(0), arg(1), arg(2)
	if r := m.collection(key, a, a0, a1, a2); r != nil {
		return r
	}
	if r := m.tryFamily(key, a, a0, a1, a2); r != nil {
		return r
	}
	if r := m.tryText(key, a, a0, a1, a2); r != nil {
		return r
	}
	if r := m.prelude(key, a, a0, a1); r != nil {
		return r
	}
	if r := m.alloc(key, a, a0, a1); r != nil {
		return r
	}
	if r := m.memStd(key, a, a0, a1); r != nil {
		return r
	}
	if r := m.mathFn(key, a0, a1); r != nil {
		return r
	}
	if r := m.bytesStd(key, a0, a1, a2); r != nil {
		return r
	}
	if strings.Contains(key, ".wrapping") || strings.Contains(key, ".rotate") {
		if r := m.intFn(key, a0, a1); r != nil {
			return r
		}
	}
	if strings.HasPrefix(key, "prelude.") && strings.Contains(key, ".to") {
		if r := m.convert(a0); r != nil {
			return r
		}
	}
	switch key {
	case "prelude.String.len", "prelude.Bytes.len":
		return mkInt(int64(len(a0.str())), 64)
	case "prelude.String.toBytes", "prelude.String.bytes":
		return mkBytes(append([]byte{}, a0.str()...))
	case "prelude.String.fromBytes":
		if !utf8.Valid(a0.str()) {
			return m.errMsg("invalid UTF-8")
		}
		return m.ok(mkStr(append([]byte{}, a0.str()...)))
	case "prelude.String.tryFromBytes":
		if !utf8.Valid(a0.str()) {
			return m.errMsg("invalid UTF-8")
		}
		if m.faultTick() {
			return m.errMsg("out of memory")
		}
		return m.ok(mkStr(append([]byte{}, a0.str()...)))
	case "prelude.String.sliceBytes":
		s, e := a1.i, a2.i
		if s < 0 || s > e || e > int64(len(a0.str())) || !boundary(a0.str(), s) || !boundary(a0.str(), e) {
			return m.none()
		}
		return m.some(mkStr(append([]byte{}, a0.str()[s:e]...)))
	// The VM has no timing guarantee to keep; this mirrors the algorithm
	// for correctness only (subtle.kg's doc comment says so).
	case "crypto/subtle.constantTimeEq":
		if len(a0.str()) != len(a1.str()) {
			return mkBool(false)
		}
		var diff byte
		for i := range a0.str() {
			diff |= a0.str()[i] ^ a1.str()[i]
		}
		return mkBool(diff == 0)
	case "prelude.Char.fromInt":
		v := a0.i
		if v < 0 || v > 0x10FFFF || (v >= 0xD800 && v <= 0xDFFF) {
			return m.none()
		}
		return m.some(mkChar(rune(v)))
	case "text.fromChar":
		return mkStr([]byte(string(a0.ch())))
	case "text.parseFloat":
		if !floatTextOK(a0.str()) {
			return m.none()
		}
		f, err := strconv.ParseFloat(string(a0.str()), 64)
		if err != nil {
			return m.none()
		}
		return m.some(mkFloat(f, 64|512))
	}
	panic(&Unsupported{What: "primitive " + key})
}

func boundary(s []byte, i int64) bool { return i == int64(len(s)) || s[i]&0xC0 != 0x80 }

func (m *Machine) some(v *obj) *obj {
	return mkVariant(m.variantOf(m.r.Types.OptionEnt(), "Some"), []*obj{v})
}

func (m *Machine) none() *obj { return mkVariant(m.variantOf(m.r.Types.OptionEnt(), "None"), nil) }

func (m *Machine) ok(v *obj) *obj {
	return mkVariant(m.variantOf(m.r.Types.ResultEnt(), "Ok"), []*obj{v})
}

func (m *Machine) errMsg(msg string) *obj {
	return mkVariant(m.variantOf(m.r.Types.ResultEnt(), "Err"), []*obj{mkOpaque("Error", msg)})
}

// allocError is Err(AllocError.OutOfMemory), for a primitive whose fault
// injection fired (rt_core.c's alloc_error mirrors this on the native side).
func (m *Machine) allocError() *obj {
	oom := mkVariant(m.variantOf(m.lookup("std/prelude", "AllocError"), "OutOfMemory"), nil)
	return mkVariant(m.variantOf(m.r.Types.ResultEnt(), "Err"), []*obj{oom})
}

// intFn is prelude.<T>.wrapping* / rotate*: modular arithmetic masked to
// the width.
func (m *Machine) intFn(key string, a0, a1 *obj) *obj {
	if !strings.HasPrefix(key, "prelude.") {
		return nil
	}
	nk := a0.nkind()
	op := key[strings.LastIndex(key, ".")+1:]
	if is128(nk) {
		return wrapping128(op, a0, a1)
	}
	b := bitsOf(nk)
	x, y := uint64(a0.i), uint64(a1.i)
	switch op {
	case "wrappingAdd":
		return mkInt(truncateTo(int64(x+y), nk), nk)
	case "wrappingSub":
		return mkInt(truncateTo(int64(x-y), nk), nk)
	case "wrappingMul":
		return mkInt(truncateTo(int64(x*y), nk), nk)
	}
	c := uint(y & uint64(b-1))
	ux := x
	if b < 64 {
		ux = x & (uint64(1)<<b - 1)
	}
	switch op {
	case "wrappingShiftLeft":
		return mkInt(truncateTo(int64(ux<<c), nk), nk)
	case "wrappingShiftRight":
		if isSigned(nk) {
			return mkInt(truncateTo(truncateTo(int64(ux), nk)>>c, nk), nk)
		}
		return mkInt(truncateTo(int64(ux>>c), nk), nk)
	case "rotateLeft":
		if c == 0 {
			return mkInt(truncateTo(int64(ux), nk), nk)
		}
		return mkInt(truncateTo(int64(ux<<c|ux>>(uint(b)-c)), nk), nk)
	case "rotateRight":
		if c == 0 {
			return mkInt(truncateTo(int64(ux), nk), nk)
		}
		return mkInt(truncateTo(int64(ux>>c|ux<<(uint(b)-c)), nk), nk)
	}
	return nil
}
