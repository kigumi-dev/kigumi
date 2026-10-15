package llgen

import (
	"fmt"
	"math/big"
	"strings"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

// wideIntrinsics are the 128-bit intrinsics LLVM expands to plain add/shift instructions on every target, with no compiler-rt call.
const wideIntrinsics = `declare { i128, i1 } @llvm.sadd.with.overflow.i128(i128, i128)
declare { i128, i1 } @llvm.uadd.with.overflow.i128(i128, i128)
declare { i128, i1 } @llvm.ssub.with.overflow.i128(i128, i128)
declare { i128, i1 } @llvm.usub.with.overflow.i128(i128, i128)
declare i128 @llvm.fshl.i128(i128, i128, i128)
declare i128 @llvm.fshr.i128(i128, i128, i128)

`

// wideConstText is the literal as the signed decimal LLVM reads back as the same 128 bits.
func wideConstText(in mir.Inst) string {
	x := new(big.Int)
	if in.Str != "" {
		x.SetString(in.Str, 10)
	} else if in.Lit.Int != nil {
		x.Set(in.Lit.Int)
	}
	mod := new(big.Int).Lsh(big.NewInt(1), 128)
	x.Mod(x, mod)
	if x.Cmp(new(big.Int).Lsh(big.NewInt(1), 127)) >= 0 {
		x.Sub(x, mod)
	}
	return x.String()
}

func (e *emitter) wideType(t sem.TypeID) bool {
	ty, ok := e.scalarType(t)
	return ok && ty == TI128
}

func (e *emitter) splitWide(sb *strings.Builder, v string) (lo, hi Value) {
	l, s, h := e.newTmp(), e.newTmp(), e.newTmp()
	fmt.Fprintf(sb, "  %s = trunc i128 %s to i64\n  %s = lshr i128 %s, 64\n  %s = trunc i128 %s to i64\n", l, v, s, v, h, s)
	return vi64text(l), vi64text(h)
}

func (e *emitter) joinWide(sb *strings.Builder, lo, hi Value) Value {
	l, h, s, r := e.newTmp(), e.newTmp(), e.newTmp(), e.newTmp()
	fmt.Fprintf(sb, "  %s = zext i64 %s to i128\n  %s = zext i64 %s to i128\n  %s = shl i128 %s, 64\n  %s = or i128 %s, %s\n", l, lo.Text, h, hi.Text, s, h, r, l, s)
	return Value{TI128, r}
}

func (e *emitter) wideInstr(sb *strings.Builder, format string, args ...any) string {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = "+format+"\n", append([]any{t}, args...)...)
	return t
}

// overflowTrap branches to the runtime's overflow panic when flag is set and carries on otherwise.
func (e *emitter) overflowTrap(sb *strings.Builder, flag, msg string) {
	bad, ok := e.label("ovf"), e.label("ovfok")
	fmt.Fprintf(sb, "  br i1 %s, label %%%s, label %%%s\n%s:\n", flag, bad, ok, bad)
	e.irb(sb).Call(rtPanic, vptr(e.cstr(msg)))
	fmt.Fprintf(sb, "  unreachable\n%s:\n", ok)
}

func (e *emitter) checkedWide(sb *strings.Builder, intrinsic string, l, r Value) Value {
	pair := e.wideInstr(sb, "call { i128, i1 } @llvm.%s.with.overflow.i128(i128 %s, i128 %s)", intrinsic, l.Text, r.Text)
	v := e.wideInstr(sb, "extractvalue { i128, i1 } %s, 0", pair)
	o := e.wideInstr(sb, "extractvalue { i128, i1 } %s, 1", pair)
	e.overflowTrap(sb, o, "integer overflow")
	return Value{TI128, v}
}

// wideViaRuntime runs op in rt_core.c, which hands the two result words back through a stack slot.
func (e *emitter) wideViaRuntime(sb *strings.Builder, fn RuntimeFn, nk int, l, r Value) Value {
	slot := e.stackSlot(sb, "[2 x i64]")
	llo, lhi := e.splitWide(sb, l.Text)
	rlo, rhi := e.splitWide(sb, r.Text)
	e.irb(sb).Call(fn, llo, lhi, rlo, rhi, vi32(nk), vptr(slot))
	hp := e.newTmp()
	fmt.Fprintf(sb, "  %s = getelementptr i64, ptr %s, i64 1\n", hp, slot)
	lo, hi := e.newTmp(), e.newTmp()
	fmt.Fprintf(sb, "  %s = load i64, ptr %s\n  %s = load i64, ptr %s\n", lo, slot, hi, hp)
	return e.joinWide(sb, vi64text(lo), vi64text(hi))
}

// Operation flags of rt_i128_mul and rt_i128_div packed above the numeric kind.
const (
	wideWrap = 1 << 16
	wideRem  = 1 << 17
)

func (e *emitter) wideShift(sb *strings.Builder, op string, signed bool, l, count Value) Value {
	bad, ok, c := e.label("shl"), e.label("shlok"), e.newTmp()
	fmt.Fprintf(sb, "  %s = icmp uge i64 %s, 128\n  br i1 %s, label %%%s, label %%%s\n%s:\n", c, count.Text, c, bad, ok, bad)
	e.irb(sb).Call(rtPanic, vptr(e.cstr("shift count out of range")))
	fmt.Fprintf(sb, "  unreachable\n%s:\n", ok)
	ext := e.wideInstr(sb, "zext i64 %s to i128", count.Text)
	return e.shiftWide(sb, op, signed, l, Value{TI128, ext})
}

func (e *emitter) shiftWide(sb *strings.Builder, op string, signed bool, l, count Value) Value {
	kind := "lshr"
	switch {
	case op == "<<":
		kind = "shl"
	case signed:
		kind = "ashr"
	}
	return Value{TI128, e.wideInstr(sb, "%s i128 %s, %s", kind, l.Text, count.Text)}
}

// wideBinaryOp is nativeBinaryOp for i128 and u128; the order operators and ==/!= are already handled by the caller's width-generic cases.
func (e *emitter) wideBinaryOp(sb *strings.Builder, op string, nk int, l, r Value) (Value, bool) {
	signed := isSignedNk(nk)
	pick := func(s, u string) string {
		if signed {
			return s
		}
		return u
	}
	switch op {
	case "<", "<=", ">", ">=":
		return Value{TI1, e.icmp(sb, cmpCC(op, signed), TI128, l, r)}, true
	case "+":
		return e.checkedWide(sb, pick("sadd", "uadd"), l, r), true
	case "-":
		return e.checkedWide(sb, pick("ssub", "usub"), l, r), true
	case "*":
		return e.wideViaRuntime(sb, rtI128Mul, nk, l, r), true
	case "/":
		return e.wideViaRuntime(sb, rtI128Div, nk, l, r), true
	case "%":
		return e.wideViaRuntime(sb, rtI128Div, nk|wideRem, l, r), true
	case "&":
		return Value{TI128, e.binOpT(sb, "and", TI128, l, r)}, true
	case "|":
		return Value{TI128, e.binOpT(sb, "or", TI128, l, r)}, true
	case "^":
		return Value{TI128, e.binOpT(sb, "xor", TI128, l, r)}, true
	case "<<", ">>":
		if r.Type == TI64 {
			return e.wideShift(sb, op, signed, l, r), true
		}
	}
	return Value{}, false
}

func (e *emitter) wideUnary(sb *strings.Builder, op string, nk int, v Value) (Value, bool) {
	switch op {
	case "-":
		zero := Value{TI128, "0"}
		if isSignedNk(nk) {
			return e.checkedWide(sb, "ssub", zero, v), true
		}
		return e.checkedWide(sb, "usub", zero, v), true
	case "~":
		return Value{TI128, e.binOpT(sb, "xor", TI128, v, Value{TI128, "-1"})}, true
	}
	return Value{}, false
}
