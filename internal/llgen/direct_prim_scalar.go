package llgen

import (
	"strings"

	"kigumi/internal/mir"
)

var (
	rtDWrapping = RuntimeFn{"rt_d_wrapping", []Type{TI32, TI64, TI64, TI32}, TI64}
	rtDSqrt     = RuntimeFn{"rt_d_sqrt", []Type{TDouble}, TDouble}
	rtDFloor    = RuntimeFn{"rt_d_floor", []Type{TDouble}, TDouble}
	rtDCeil     = RuntimeFn{"rt_d_ceil", []Type{TDouble}, TDouble}
	rtDRound    = RuntimeFn{"rt_d_round", []Type{TDouble}, TDouble}
	rtDPow      = RuntimeFn{"rt_d_pow", []Type{TDouble, TDouble}, TDouble}
)

var wrappingOp = map[mir.DirectPrim]int{
	mir.DPWrapAdd: 0, mir.DPWrapSub: 1, mir.DPWrapMul: 2, mir.DPWrapShl: 3, mir.DPWrapShr: 4, mir.DPRotl: 5, mir.DPRotr: 6,
}

var mathFns = map[mir.DirectPrim]RuntimeFn{
	mir.DPSqrt: rtDSqrt, mir.DPFloor: rtDFloor, mir.DPCeil: rtDCeil, mir.DPRound: rtDRound, mir.DPPow: rtDPow,
}

func (e *emitter) scalarPrimCall(sb *strings.Builder, f *mir.Func, in mir.Inst, args []Value) bool {
	b := e.irb(sb)
	if op, ok := wrappingOp[in.DPrim]; ok {
		nk := e.numKind(f.Locals[in.Dst].Type)
		e.storeScalarResult(sb, f, in.Dst, b.Call(rtDWrapping, vi32(op), args[0], args[1], vi32(nk)))
		return true
	}
	if fn, ok := mathFns[in.DPrim]; ok {
		e.storeScalarResult(sb, f, in.Dst, b.Call(fn, args...))
		return true
	}
	return false
}
