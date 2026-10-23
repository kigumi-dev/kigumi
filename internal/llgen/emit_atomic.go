package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
)

var llvmOrder = []string{"monotonic", "acquire", "release", "acq_rel", "seq_cst"}

// A cmpxchg failure order is never release and never stronger than the success order.
var failOrder = []string{"monotonic", "acquire", "monotonic", "acquire", "seq_cst"}

func isAtomicBuiltin(in mir.Inst) bool {
	return in.Op == mir.OpBuiltin && strings.HasPrefix(in.Str, "atomic.")
}

func (e *emitter) tryAtomicInst(sb *strings.Builder, f *mir.Func, in mir.Inst) bool {
	if !isAtomicBuiltin(in) {
		return false
	}
	slot, ok := e.directStatic(in.Ent)
	if !ok {
		return false
	}
	op := strings.TrimPrefix(in.Str, "atomic.")
	args := []string{"ptr " + e.globalSym(in.Ent)}
	for _, a := range in.Args {
		v, _ := e.scalarOperand(sb, f, a)
		if v.Type == TI1 {
			v = Value{TI8, e.zext(sb, v.Text, "i1", "i8")}
		}
		args = append(args, fmt.Sprintf("%s %s", v.Type, v.Text))
	}
	fn := e.atomicHelper(op, slot, e.numKind(e.r.Entity(in.Ent).Type), in.Index)
	ret := "void"
	if op != "store" {
		ret = "i64"
		if slot == TI1 {
			ret = "i8"
		}
	}
	res := e.newTmp()
	if ret == "void" {
		fmt.Fprintf(sb, "  call void %s(%s)\n", fn, strings.Join(args, ", "))
		e.store(sb, in.Dst, e.irb(sb).Call(rtUnit).Text)
		return true
	}
	fmt.Fprintf(sb, "  %s = call %s %s(%s)\n", res, ret, fn, strings.Join(args, ", "))
	if slot == TI1 {
		t := e.newTmp()
		fmt.Fprintf(sb, "  %s = trunc i8 %s to i1\n", t, res)
		res = t
	}
	e.storeScalarResult(sb, f, in.Dst, Value{slot, res})
	return true
}

func (e *emitter) zext(sb *strings.Builder, v, from, to string) string {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = zext %s %s to %s\n", t, from, v, to)
	return t
}

func (e *emitter) atomicHelper(op string, slot Type, nk, order int) string {
	sym := fmt.Sprintf("@\"kg.atomic.%s.%s.%d.%s\"", op, slot, nk, llvmOrder[order])
	if e.cio[sym] {
		return sym
	}
	e.cio[sym] = true
	ty := "i64"
	if slot == TI1 {
		ty = "i8"
	}
	ord, fail := llvmOrder[order], failOrder[order]
	var body, params string
	switch op {
	case "load":
		params = ""
		body = fmt.Sprintf("  %%r = load atomic %s, ptr %%p %s, align %d\n  ret %s %%r\n", ty, ord, alignOf(ty), ty)
	case "store":
		params = fmt.Sprintf(", %s %%v", ty)
		body = fmt.Sprintf("  store atomic %s %%v, ptr %%p %s, align %d\n  ret void\n", ty, ord, alignOf(ty))
	case "exchange":
		params = fmt.Sprintf(", %s %%v", ty)
		body = fmt.Sprintf("  %%r = atomicrmw xchg ptr %%p, %s %%v %s\n  ret %s %%r\n", ty, ord, ty)
	case "compareExchange":
		params = fmt.Sprintf(", %s %%x, %s %%v", ty, ty)
		body = fmt.Sprintf("  %%pair = cmpxchg ptr %%p, %s %%x, %s %%v %s %s\n  %%r = extractvalue { %s, i1 } %%pair, 0\n  ret %s %%r\n", ty, ty, ord, fail, ty, ty)
	case "add", "sub":
		params = ", i64 %v"
		if nk&255 == 64 {
			body = fmt.Sprintf("  %%r = atomicrmw %s ptr %%p, i64 %%v %s\n  ret i64 %%r\n", op, ord)
		} else {
			body = e.narrowRMW(op, nk, ord, fail)
		}
	default:
		params = ", i64 %v"
		body = fmt.Sprintf("  %%r = atomicrmw %s ptr %%p, i64 %%v %s\n  ret i64 %%r\n", op, ord)
	}
	ret := ty
	if op == "store" {
		ret = "void"
	}
	fmt.Fprintf(&e.helpers, "define internal %s %s(ptr %%p%s) nounwind {\nentry:\n%s}\n\n", ret, sym, params, body)
	return sym
}

func alignOf(ty string) int {
	if ty == "i8" {
		return 1
	}
	return 8
}

// One atomicrmw cannot wrap the sum at a narrow type's width, so retry a compare-exchange.
func (e *emitter) narrowRMW(op string, nk int, ord, fail string) string {
	bits := nk & 255
	wrap := fmt.Sprintf("  %%new = and i64 %%raw, %d\n", uint64(1)<<bits-1)
	if nk&256 != 0 {
		wrap = fmt.Sprintf("  %%up = shl i64 %%raw, %d\n  %%new = ashr i64 %%up, %d\n", 64-bits, 64-bits)
	}
	return fmt.Sprintf("  %%first = load atomic i64, ptr %%p monotonic, align 8\n  br label %%loop\nloop:\n  %%old = phi i64 [ %%first, %%entry ], [ %%cur, %%loop ]\n  %%raw = %s i64 %%old, %%v\n%s  %%pair = cmpxchg ptr %%p, i64 %%old, i64 %%new %s %s\n  %%cur = extractvalue { i64, i1 } %%pair, 0\n  %%ok = extractvalue { i64, i1 } %%pair, 1\n  br i1 %%ok, label %%done, label %%loop\ndone:\n  ret i64 %%old\n", op, wrap, ord, fail)
}
