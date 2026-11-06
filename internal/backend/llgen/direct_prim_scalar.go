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
	if in.DPrim == opdef.DPPow {
		e.storeScalarResult(sb, f, in.Dst, Value{Type: TDouble, Text: e.libmPow(sb, args[0].Text, args[1].Text)})
		return true
	}
	if name, ok := mathIntrinsics[in.DPrim]; ok {
		texts := make([]string, len(args))
		for i, a := range args {
			texts[i] = a.Text
		}
		e.storeScalarResult(sb, f, in.Dst, Value{Type: TDouble, Text: e.callIntrinsic(sb, name, "double", texts...)})
		return true
	}
	return false
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

// libmPow calls libm's pow as nobuiltin: pow is not correctly rounded, so LLVM must not fold it with the compiler's own libm.
func (e *emitter) libmPow(sb *strings.Builder, x, y string) string {
	e.foreign["pow"] = "declare double @pow(double, double)"
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = call double @pow(double %s, double %s) nobuiltin\n", t, x, y)
	return t
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
