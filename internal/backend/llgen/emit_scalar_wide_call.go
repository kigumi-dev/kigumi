package llgen

import (
	"strings"

	"kigumi/internal/compiler/mir"
	opdef "kigumi/internal/compiler/mir/ops"
)

// wideCallOp names the 128-bit prelude primitive in a call computed on registers, or "".
func (e *emitter) wideCallOp(f *mir.Func, in mir.Inst) string {
	if in.Op != mir.OpCall || len(in.Args) != 2 || !e.wideType(f.Locals[in.Dst].Type) {
		return ""
	}
	for _, a := range in.Args {
		if !e.wideType(f.Locals[a].Type) {
			return ""
		}
	}
	id, ok := in.StdOp()
	if !ok {
		return ""
	}
	switch name := opdef.StdOpNames[id]; name {
	case "WRAPPING_ADD", "WRAPPING_SUB", "WRAPPING_MUL", "WRAPPING_SHL", "WRAPPING_SHR", "ROTATE_LEFT", "ROTATE_RIGHT":
		return name
	}
	return ""
}

func (e *emitter) wideCall(sb *strings.Builder, f *mir.Func, in mir.Inst, op string) bool {
	l, _ := e.scalarOperand(sb, f, in.Args[0])
	r, _ := e.scalarOperand(sb, f, in.Args[1])
	nk := e.numKind(f.Locals[in.Dst].Type)
	signed := isSignedNk(nk)
	var res Value
	switch op {
	case "WRAPPING_ADD":
		res = Value{TI128, e.binOpT(sb, "add", TI128, l, r)}
	case "WRAPPING_SUB":
		res = Value{TI128, e.binOpT(sb, "sub", TI128, l, r)}
	case "WRAPPING_MUL":
		res = Value{TI128, e.binOpT(sb, "mul", TI128, l, r)}
	case "WRAPPING_SHL", "WRAPPING_SHR":
		c := Value{TI128, e.binOpT(sb, "and", TI128, r, Value{TI128, "127"})}
		res = e.shiftWide(sb, map[string]string{"WRAPPING_SHL": "<<", "WRAPPING_SHR": ">>"}[op], signed, l, c)
	default:
		fn := map[string]string{"ROTATE_LEFT": "fshl.i128", "ROTATE_RIGHT": "fshr.i128"}[op]
		res = Value{TI128, e.callIntrinsic(sb, fn, "i128", l.Text, l.Text, r.Text)}
	}
	e.storeScalarResult(sb, f, in.Dst, res)
	return true
}
