package llgen

import (
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
)

func (e *emitter) rawConvert(f *midir.Func, in midir.Inst) bool { return e.p.FloatConversion(f, in) }

func (e *emitter) rawConvertInst(sb *strings.Builder, f *midir.Func, in midir.Inst) bool {
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
