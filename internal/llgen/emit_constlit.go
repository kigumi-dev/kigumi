package llgen

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// Object kinds of runtime/rt.h; constlit_test.go checks them against it.
const (
	kInt   = 1
	kFloat = 2
	kStr   = 4
	kBytes = 5
	kChar  = 6
)

// constLit is a literal: its value is fixed before the program starts, so
// one immortal stands for every site that builds it.
type constLit struct {
	kind int
	nk   int
	num  string
	data string
	len  int
}

// constNum reserves the immortal for an integer, float or char literal.
// num is the i64 (or raw double bits) the runtime writes into the slot.
func (e *emitter) constNum(kind, nk int, num string) string {
	key := fmt.Sprintf("n%d:%d:%s", kind, nk, num)
	e.registerConst(key, constVal{lit: &constLit{kind: kind, nk: nk, num: num}})
	return key
}

// constText reserves the immortal for a string literal, whose bytes
// already live in the read-only data cstr emits.
func (e *emitter) constText(kind int, s string) string {
	key := fmt.Sprintf("t%d:%d:%s", kind, len(s), s)
	e.registerConst(key, constVal{lit: &constLit{kind: kind, data: e.cstr(s), len: len(s)}})
	return key
}

// initLit writes one literal into its slot, from kg_init_immortals.
func (e *emitter) initLit(sb *strings.Builder, slot string, lit *constLit) {
	if lit.kind == kStr {
		e.irb(sb).Call(rtConstText, vptr(slot), vi32(lit.kind), vptr(lit.data), vi64(lit.len))
		return
	}
	num, f := vi64text(lit.num), vdouble("0.0")
	if lit.kind == kFloat {
		num, f = vi64(0), vdouble(lit.num)
	}
	if lit.kind == kInt && lit.nk&255 == 128 {
		lo, hi := int128Words(lit.num)
		e.irb(sb).Call(rtConstInt128, vptr(slot), vi32(lit.nk), vi64text(lo), vi64text(hi))
		return
	}
	e.irb(sb).Call(rtConstNum, vptr(slot), vi32(lit.kind), vi32(lit.nk), num, f)
}

// int128Words splits a decimal literal into the low and high words, in two's complement.
func int128Words(text string) (lo, hi string) {
	x, _ := new(big.Int).SetString(text, 10)
	m := new(big.Int).And(x, new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1)))
	low := new(big.Int).And(m, new(big.Int).SetUint64(math.MaxUint64)).Uint64()
	return strconv.FormatInt(int64(low), 10), strconv.FormatInt(int64(new(big.Int).Rsh(m, 64).Uint64()), 10)
}
