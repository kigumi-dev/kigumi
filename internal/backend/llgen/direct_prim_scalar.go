package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/compiler/mir"
	opdef "kigumi/internal/compiler/mir/ops"
)

var mathIntrinsics = map[opdef.DirectPrim]string{
	opdef.DPSqrt: "sqrt.f64", opdef.DPFloor: "floor.f64", opdef.DPCeil: "ceil.f64", opdef.DPRound: "round.f64",
}

func (e *emitter) scalarPrimCall(sb *strings.Builder, f *mir.Func, in mir.Inst, args []Value) bool {
	if in.DPrim >= opdef.DPWrapAdd && in.DPrim <= opdef.DPRotr {
		nk := e.numKind(f.Locals[in.Dst].Type)
		e.storeScalarResult(sb, f, in.Dst, e.wrappingOp(sb, in.DPrim, nk, args[0], args[1]))
		return true
	}
	if in.DPrim == opdef.DPPow {
		e.storeScalarResult(sb, f, in.Dst, Value{TDouble, e.libmPow(sb, args[0].Text, args[1].Text)})
		return true
	}
	if name, ok := mathIntrinsics[in.DPrim]; ok {
		texts := make([]string, len(args))
		for i, a := range args {
			texts[i] = a.Text
		}
		e.storeScalarResult(sb, f, in.Dst, Value{TDouble, e.callIntrinsic(sb, name, "double", texts...)})
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
