package llgen

import (
	"fmt"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
)

var mathIntrinsics = map[opdef.DirectPrim]string{
	opdef.DPSqrt: "sqrt.f64", opdef.DPFloor: "floor.f64", opdef.DPCeil: "ceil.f64", opdef.DPRound: "round.f64",
}

func (e *emitter) scalarPrimCall(sb *strings.Builder, f *midir.Func, in midir.Inst, args []Value) bool {
	if in.DPrim >= opdef.DPWrapAdd && in.DPrim <= opdef.DPRotr {
		nk := e.numKind(f.Locals[in.Dst].Type)
		e.storeScalarResult(sb, f, in.Dst, e.wrappingOp(sb, in.DPrim, nk, args[0], args[1]))
		return true
	}
	if in.DPrim >= opdef.DPSecretBarrier && in.DPrim <= opdef.DPSecretSelect {
		x := args[0].Text
		switch in.DPrim {
		case opdef.DPSecretBarrier:
			e.storeScalarResult(sb, f, in.Dst, vi64text(e.secretBarrier(sb, x)))
		case opdef.DPSecretMaskNonZero:
			t := e.newTmp()
			fmt.Fprintf(sb, "  %s = icmp ne i64 %s, 0\n", t, x)
			e.storeScalarResult(sb, f, in.Dst, vi64text(e.secretMask(sb, t, true)))
		case opdef.DPSecretMaskLt:
			t := e.newTmp()
			fmt.Fprintf(sb, "  %s = icmp ult i64 %s, %s\n", t, x, args[1].Text)
			e.storeScalarResult(sb, f, in.Dst, vi64text(e.secretMask(sb, t, true)))
		case opdef.DPSecretSelect:
			e.storeScalarResult(sb, f, in.Dst, vi64text(e.secretSelect(sb, x, args[1].Text, args[2].Text)))
		}
		return true
	}
	if in.DPrim == opdef.DPToBits || in.DPrim == opdef.DPFromBits {
		e.storeScalarResult(sb, f, in.Dst, e.floatBitcast(sb, in.DPrim, args[0].Text))
		return true
	}
	if name, ok := mathIntrinsics[in.DPrim]; ok {
		texts := make([]string, len(args))
		for i, a := range args {
			texts[i] = a.Text
		}
		v := e.callIntrinsic(sb, name, "double", texts...)
		if in.DPrim != opdef.DPSqrt {
			v = e.quietNaNArgument(sb, texts[0], v)
		}
		e.storeScalarResult(sb, f, in.Dst, Value{Type: TDouble, Text: v})
		return true
	}
	return false
}

// quietNaNArgument makes a rounding of NaN return the argument quiet with its payload; libm's floor returns a signaling
// NaN as it is, and the choice is made on the bits because LLVM treats NaN payloads in float selects as free.
func (e *emitter) quietNaNArgument(sb *strings.Builder, x, rounded string) string {
	b, r, sh, isNaN, q, sel, out := e.newTmp(), e.newTmp(), e.newTmp(), e.newTmp(), e.newTmp(), e.newTmp(), e.newTmp()
	fmt.Fprintf(sb, "  %s = bitcast double %s to i64\n  %s = bitcast double %s to i64\n", b, x, r, rounded)
	fmt.Fprintf(sb, "  %s = shl i64 %s, 1\n  %s = icmp ugt i64 %s, -9007199254740992\n", sh, b, isNaN, sh)
	fmt.Fprintf(sb, "  %s = or i64 %s, 2251799813685248\n  %s = select i1 %s, i64 %s, i64 %s\n", q, b, sel, isNaN, q, r)
	fmt.Fprintf(sb, "  %s = bitcast i64 %s to double\n", out, sel)
	return out
}

func (e *emitter) wrappingOp(sb *strings.Builder, prim opdef.DirectPrim, nk int, l, r Value) Value {
	ty := intTy(nk)
	x, y := e.narrowInt(sb, nk, l.Text), e.narrowInt(sb, nk, r.Text)
	instr := func(op, a, b string) string {
		t := e.newTmp()
		fmt.Fprintf(sb, "  %s = %s %s %s, %s\n", t, op, ty, a, b)
		return t
	}
	var v string
	switch prim {
	case opdef.DPWrapAdd:
		v = instr("add", x, y)
	case opdef.DPWrapSub:
		v = instr("sub", x, y)
	case opdef.DPWrapMul:
		v = instr("mul", x, y)
	case opdef.DPWrapShl, opdef.DPWrapShr:
		c := instr("and", y, fmt.Sprint(intBits(nk)-1))
		op := "shl"
		if prim == opdef.DPWrapShr {
			op = "lshr"
			if isSignedNk(nk) {
				op = "ashr"
			}
		}
		v = instr(op, x, c)
	case opdef.DPRotl:
		v = e.callIntrinsic(sb, "fshl."+ty, ty, x, x, y)
	default:
		v = e.callIntrinsic(sb, "fshr."+ty, ty, x, x, y)
	}
	return vi64text(e.widenInt(sb, nk, v))
}

func (e *emitter) floatBitcast(sb *strings.Builder, prim opdef.DirectPrim, x string) Value {
	t := e.newTmp()
	if prim == opdef.DPToBits {
		fmt.Fprintf(sb, "  %s = bitcast double %s to i64\n", t, x)
		return vi64text(t)
	}
	fmt.Fprintf(sb, "  %s = bitcast i64 %s to double\n", t, x)
	return vdouble(t)
}

// secretBarrier keeps the optimiser from folding a mask back into a branch.
func (e *emitter) secretBarrier(sb *strings.Builder, x string) string {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = call i64 asm sideeffect \"\", \"=r,0\"(i64 %s)\n", t, x)
	return t
}

// secretMask turns x (a boolean or masked value) into an all-ones/all-zeros mask.
func (e *emitter) secretMask(sb *strings.Builder, x string, isBool ...bool) string {
	v := x
	if len(isBool) > 0 {
		z := e.newTmp()
		fmt.Fprintf(sb, "  %s = zext i1 %s to i64\n", z, x)
		v = z
	}
	n := e.newTmp()
	fmt.Fprintf(sb, "  %s = sub i64 0, %s\n", n, v)
	return e.secretBarrier(sb, n)
}

func (e *emitter) secretSelect(sb *strings.Builder, m, a, b string) string {
	mm := e.secretBarrier(sb, m)
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = and i64 %s, %s\n", t, mm, a)
	nm := e.newTmp()
	fmt.Fprintf(sb, "  %s = xor i64 %s, -1\n", nm, mm)
	u := e.newTmp()
	fmt.Fprintf(sb, "  %s = and i64 %s, %s\n", u, nm, b)
	r := e.newTmp()
	fmt.Fprintf(sb, "  %s = or i64 %s, %s\n", r, t, u)
	return e.secretBarrier(sb, r)
}
