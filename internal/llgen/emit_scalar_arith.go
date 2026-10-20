package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
)

// scalarBinary checks both operands are scalar-eligible; the MIR type
// checker never mixes register kinds within one binary op.
func (e *emitter) scalarBinary(sb *strings.Builder, f *mir.Func, in mir.Inst) bool {
	lt := e.operandType(f.Locals[in.Args[0]].Type)
	lty, lok := e.scalarType(lt)
	rty, rok := e.scalarType(e.operandType(f.Locals[in.Args[1]].Type))
	if !lok || !rok || lty != rty && !wideShiftOperands(in.Str, lty, rty) {
		return false
	}
	l, _ := e.scalarOperand(sb, f, in.Args[0])
	r, _ := e.scalarOperand(sb, f, in.Args[1])
	res, ok := e.scalarBinaryOp(sb, in, lty, e.numKind(lt), l, r)
	if !ok {
		return false
	}
	e.storeScalarResult(sb, f, in.Dst, res)
	return true
}

// scalarUnary computes OpUnary's non-"cast" operators (!, -, ~) natively;
// cast is excluded upstream (classifyArgs and tryScalarInst).
func (e *emitter) scalarUnary(sb *strings.Builder, f *mir.Func, in mir.Inst) bool {
	t := e.operandType(f.Locals[in.Args[0]].Type)
	ty, ok := e.scalarType(t)
	if !ok {
		return false
	}
	v, _ := e.scalarOperand(sb, f, in.Args[0])
	nk := e.numKind(t)
	var res Value
	switch {
	case ty == TI1 && in.Str == "!":
		res = Value{TI1, e.xorI1(sb, v.Text)}
	case ty == TI128:
		var ok bool
		if res, ok = e.wideUnary(sb, in.Str, nk, v); !ok {
			return false
		}
	case ty == TI64 && in.Str == "-":
		res = Value{TI64, e.irb(sb).Call(rtIneg, v, vi32(nk)).Text}
	case ty == TI64 && in.Str == "~":
		res = Value{TI64, e.irb(sb).Call(rtInot, v, vi32(nk)).Text}
	case (ty == TDouble || ty == TFloat) && in.Str == "-":
		res = Value{ty, e.fneg(sb, ty, v.Text)}
	default:
		return false
	}
	e.storeScalarResult(sb, f, in.Dst, res)
	return true
}

// nativeBinaryOp mirrors rt_binop's dispatch: "==" and "!=" work on every
// scalar kind, the rest need TI64 (mirrored by rt_i*) or TDouble.
func (e *emitter) nativeBinaryOp(sb *strings.Builder, op string, ty Type, nk int, l, r Value) (Value, bool) {
	if ty == TDouble || ty == TFloat {
		return e.nativeFloatOp(sb, op, ty, l, r)
	}
	switch op {
	case "==":
		return Value{TI1, e.icmp(sb, "eq", ty, l, r)}, true
	case "!=":
		return Value{TI1, e.icmp(sb, "ne", ty, l, r)}, true
	}
	if ty == TI128 {
		return e.wideBinaryOp(sb, op, nk, l, r)
	}
	if ty != TI64 {
		return Value{}, false
	}
	signed := isSignedNk(nk)
	switch op {
	case "<", "<=", ">", ">=":
		return Value{TI1, e.icmp(sb, cmpCC(op, signed), ty, l, r)}, true
	case "+":
		return Value{TI64, e.irb(sb).Call(rtIadd, l, r, vi32(nk)).Text}, true
	case "-":
		return Value{TI64, e.irb(sb).Call(rtIsub, l, r, vi32(nk)).Text}, true
	case "*":
		return Value{TI64, e.irb(sb).Call(rtImul, l, r, vi32(nk)).Text}, true
	case "/":
		return Value{TI64, e.irb(sb).Call(rtIdiv, l, r, vi32(nk)).Text}, true
	case "%":
		return Value{TI64, e.irb(sb).Call(rtImod, l, r, vi32(nk)).Text}, true
	case "&":
		return Value{TI64, e.binOp(sb, "and", l, r)}, true
	case "|":
		return Value{TI64, e.binOp(sb, "or", l, r)}, true
	case "^":
		return Value{TI64, e.irb(sb).Call(rtIxor, l, r, vi32(nk)).Text}, true
	case "<<":
		return Value{TI64, e.irb(sb).Call(rtIshl, l, r, vi32(nk)).Text}, true
	case ">>":
		return Value{TI64, e.irb(sb).Call(rtIshr, l, r, vi32(nk)).Text}, true
	}
	return Value{}, false
}

// nativeFloatOp: rt_binop never checks a float result's bounds, so these
// are plain IEEE double instructions with no call at all.
func (e *emitter) nativeFloatOp(sb *strings.Builder, op string, ty Type, l, r Value) (Value, bool) {
	switch op {
	case "==":
		return Value{TI1, e.fcmp(sb, "oeq", ty, l, r)}, true
	case "!=":
		// rt_binop's "!=" is !(a==b) in C, true for NaN; "une" (not "one")
		// matches that since it treats unordered as not-equal.
		return Value{TI1, e.fcmp(sb, "une", ty, l, r)}, true
	case "<":
		return Value{TI1, e.fcmp(sb, "olt", ty, l, r)}, true
	case "<=":
		return Value{TI1, e.fcmp(sb, "ole", ty, l, r)}, true
	case ">":
		return Value{TI1, e.fcmp(sb, "ogt", ty, l, r)}, true
	case ">=":
		return Value{TI1, e.fcmp(sb, "oge", ty, l, r)}, true
	case "+":
		return Value{ty, e.binOpT(sb, "fadd", ty, l, r)}, true
	case "-":
		return Value{ty, e.binOpT(sb, "fsub", ty, l, r)}, true
	case "*":
		return Value{ty, e.binOpT(sb, "fmul", ty, l, r)}, true
	case "/":
		return Value{ty, e.binOpT(sb, "fdiv", ty, l, r)}, true
	}
	return Value{}, false
}

// isSignedNk reads mir.NumKind's sign bit (bit 8).
func isSignedNk(nk int) bool { return nk&(1<<8) != 0 }

func cmpCC(op string, signed bool) string {
	table := map[string][2]string{
		"<":  {"slt", "ult"},
		"<=": {"sle", "ule"},
		">":  {"sgt", "ugt"},
		">=": {"sge", "uge"},
	}
	cc := table[op]
	if signed {
		return cc[0]
	}
	return cc[1]
}

func (e *emitter) xorI1(sb *strings.Builder, x string) string {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = xor i1 %s, true\n", t, x)
	return t
}

func (e *emitter) fneg(sb *strings.Builder, ty Type, x string) string {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = fneg %s %s\n", t, ty, x)
	return t
}

func (e *emitter) binOp(sb *strings.Builder, op string, l, r Value) string {
	return e.binOpT(sb, op, TI64, l, r)
}

func (e *emitter) binOpT(sb *strings.Builder, op string, ty Type, l, r Value) string {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = %s %s %s, %s\n", t, op, ty, l.Text, r.Text)
	return t
}

func (e *emitter) icmp(sb *strings.Builder, cc string, ty Type, l, r Value) string {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = icmp %s %s %s, %s\n", t, cc, ty, l.Text, r.Text)
	return t
}

func (e *emitter) fcmp(sb *strings.Builder, cc string, ty Type, l, r Value) string {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = fcmp %s %s %s, %s\n", t, cc, ty, l.Text, r.Text)
	return t
}

// A boxed f32 holds the double nearest its single value, so widening and narrowing lose nothing.
func (e *emitter) extendFloat(sb *strings.Builder, x string) string {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = fpext float %s to double\n", t, x)
	return t
}

func (e *emitter) narrowFloat(sb *strings.Builder, x string) string {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = fptrunc double %s to float\n", t, x)
	return t
}
