package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
)

// ffiArgs reads a primitive's arguments: from registers on a call, from boxes in a function value's adapter.
type ffiArgs struct {
	reg func(i int, ty Type) Value
	c   func(i int, t sem.TypeID, ct string) string
	ty  func(i int) sem.TypeID
}

// ffiLowering computes a std/ffi pointer primitive on registers; a TVoid result is a store's Unit.
type ffiLowering func(e *emitter, sb *strings.Builder, a ffiArgs, dst sem.TypeID) Value

var ffiPrims = map[string]ffiLowering{
	"std/ffi.null":     ffiNull,
	"std/ffi.isNull":   ffiIsNull,
	"std/ffi.cast":     ffiSamePtr,
	"std/ffi.toConst":  ffiSamePtr,
	"std/ffi.toMut":    ffiSamePtr,
	"std/ffi.offset":   ffiOffset,
	"std/ffi.readU8":   ffiRead,
	"std/ffi.readI32":  ffiRead,
	"std/ffi.readI64":  ffiRead,
	"std/ffi.readF64":  ffiRead,
	"std/ffi.readPtr":  ffiRead,
	"std/ffi.writeU8":  ffiWrite,
	"std/ffi.writeI32": ffiWrite,
	"std/ffi.writeI64": ffiWrite,
	"std/ffi.writeF64": ffiWrite,
	"std/ffi.writePtr": ffiWrite,
}

func ffiNull(*emitter, *strings.Builder, ffiArgs, sem.TypeID) Value { return Value{TCPtr, "null"} }

func ffiIsNull(e *emitter, sb *strings.Builder, a ffiArgs, _ sem.TypeID) Value {
	c := e.newTmp()
	fmt.Fprintf(sb, "  %s = icmp eq ptr %s, null\n", c, a.reg(0, TCPtr).Text)
	return Value{TI1, c}
}

func ffiSamePtr(_ *emitter, _ *strings.Builder, a ffiArgs, _ sem.TypeID) Value {
	return Value{TCPtr, a.reg(0, TCPtr).Text}
}

func ffiOffset(e *emitter, sb *strings.Builder, a ffiArgs, _ sem.TypeID) Value {
	base, n := a.reg(0, TCPtr), a.reg(1, TI64)
	p := e.newTmp()
	fmt.Fprintf(sb, "  %s = getelementptr i8, ptr %s, i64 %s\n", p, base.Text, n.Text)
	return Value{TCPtr, p}
}

func ffiRead(e *emitter, sb *strings.Builder, a ffiArgs, dst sem.TypeID) Value {
	ct, raw := e.cType(dst), e.newTmp()
	fmt.Fprintf(sb, "  %s = load %s, ptr %s\n", raw, ct, a.reg(0, TCPtr).Text)
	return e.cWiden(sb, raw, dst, ct)
}

func ffiWrite(e *emitter, sb *strings.Builder, a ffiArgs, _ sem.TypeID) Value {
	t := a.ty(1)
	ct := e.cType(t)
	v := a.c(1, t, ct)
	fmt.Fprintf(sb, "  store %s %s, ptr %s\n", ct, v, a.reg(0, TCPtr).Text)
	return Value{}
}

func (e *emitter) ffiRegOp(in mir.Inst) string {
	if in.Op != mir.OpCall || in.DPrim != mir.DPNone || e.r.Entity(in.Ent).Kind != sem.EntFn || e.r.Fn(in.Ent).Body != 0 {
		return ""
	}
	if key := e.stdKey(in.Ent); ffiPrims[key] != nil {
		return key
	}
	return ""
}

// ffiRegCall reads a register argument in place and boxes the result only for a boxed destination.
func (e *emitter) ffiRegCall(sb *strings.Builder, f *mir.Func, in mir.Inst, key string) {
	ops := newCOperands(in)
	a := ffiArgs{
		reg: func(i int, ty Type) Value { return ops.reg(e, sb, i, ty) },
		c:   func(i int, t sem.TypeID, ct string) string { return ops.c(e, sb, i, t, ct) },
		ty:  func(i int) sem.TypeID { return f.Locals[in.Args[i]].Type },
	}
	if v := ffiPrims[key](e, sb, a, f.Locals[in.Dst].Type); v.Type != TVoid {
		e.storeScalarResult(sb, f, in.Dst, v)
	} else {
		e.storeMaybeScalar(sb, in.Dst, e.irb(sb).Call(rtUnit).Text)
	}
	ops.release(e, sb)
}

// ffiScalarCall is the same lowering over boxed arguments, for a primitive called through a function value.
func (e *emitter) ffiScalarCall(sb *strings.Builder, key string, args []string, types []sem.TypeID, dst sem.TypeID) (string, bool) {
	lower := ffiPrims[key]
	if lower == nil {
		return "", false
	}
	a := ffiArgs{
		reg: func(i int, ty Type) Value { return e.unboxScalar(sb, ty, args[i]) },
		c:   func(i int, t sem.TypeID, ct string) string { return e.unbox(sb, args[i], t, ct) },
		ty:  func(i int) sem.TypeID { return types[i] },
	}
	if v := lower(e, sb, a, dst); v.Type != TVoid {
		return e.boxNative(sb, scalarInfo{Ty: v.Type, Nk: e.numKind(dst)}, v), true
	}
	return e.irb(sb).Call(rtUnit).Text, true
}
