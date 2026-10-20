package llgen

import (
	"strings"

	"kigumi/internal/mir"
)

func (e *emitter) rawConvert(f *mir.Func, in mir.Inst) bool { return e.p.FloatConversion(f, in) }

func (e *emitter) rawConvertInst(sb *strings.Builder, f *mir.Func, in mir.Inst) bool {
	src, _ := e.scalarOperand(sb, f, in.Args[0])
	res := src
	if src.Type == TI64 {
		op := "uitofp"
		if isSignedNk(e.numKind(e.operandType(f.Locals[in.Args[0]].Type))) {
			op = "sitofp"
		}
		res = Value{TDouble, e.wideInstr(sb, "%s i64 %s to double", op, src.Text)}
	}
	e.storeScalarResult(sb, f, in.Dst, res)
	return true
}
