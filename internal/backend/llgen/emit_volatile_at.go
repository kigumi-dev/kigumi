package llgen

import (
	"fmt"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// volatileReadCall is the generic `mem.volatileRead` call, which lowers to one
// volatile load; a scalar result can stay in a register instead of a box.
func (e *emitter) volatileReadCall(in midir.Inst) bool {
	return in.Op == midir.OpCall && len(in.Args) == 1 && e.stdKey(in.Ent) == "std/mem.volatileRead"
}

// volatileReadNative emits the load straight into dst's register, taking a
// register pointer directly or unboxing the pointer the call received.
func (e *emitter) volatileReadNative(sb *strings.Builder, f *midir.Func, in midir.Inst) {
	var p string
	if e.isScalar(in.Args[0]) {
		p = e.loadNative(sb, in.Args[0]).Text
	} else {
		p = e.irb(sb).Call(rtPtrVal, vptr(e.load(sb, in.Args[0]))).Text
	}
	info := e.scalar[in.Dst]
	dst := f.Locals[in.Dst].Type
	ty, align := e.nativeVolatileWidth(dst, info)
	raw := e.newTmp()
	fmt.Fprintf(sb, "  %s = load volatile %s, ptr %s, align %d\n", raw, ty, p, align)
	v := Value{Type: ty, Text: raw}
	switch {
	case info.Ty == TI1:
		t := e.newTmp()
		fmt.Fprintf(sb, "  %s = trunc i8 %s to i1\n", t, raw)
		v = Value{Type: TI1, Text: t}
	case info.Ty == TI64 && ty != TI64:
		ext := "zext"
		if e.r.Types.IsSigned(dst) {
			ext = "sext"
		}
		t := e.newTmp()
		fmt.Fprintf(sb, "  %s = %s %s %s to i64\n", t, ext, ty, raw)
		v = Value{Type: TI64, Text: t}
	}
	e.storeNative(sb, in.Dst, v)
}

// nativeVolatileWidth is the in-memory width of a volatile value: a register
// scalar keeps its own type, Bool loads as one byte, pointers as ptr.
func (e *emitter) nativeVolatileWidth(t sem.TypeID, info scalarInfo) (Type, int) {
	switch info.Ty {
	case TI1:
		return TI8, 1
	case TI32:
		return TI32, 4
	case TDouble:
		return TDouble, 8
	case TFloat:
		return TFloat, 4
	case TCPtr:
		return TCPtr, 8
	}
	bits := e.r.Types.Width(t)
	var ty Type
	switch bits {
	case 8:
		ty = TI8
	case 16:
		ty = TI16
	case 32:
		ty = TI32
	case 128:
		ty = TI128
	default:
		ty = TI64
	}
	return ty, max(bits/8, 1)
}

func (e *emitter) tryVolatileAt(sb *strings.Builder, f *midir.Func, in midir.Inst) bool {
	if in.Op != midir.OpBuiltin || in.Str != "volatile.read" && in.Str != "volatile.write" {
		return false
	}
	addr, _ := e.scalarOperand(sb, f, in.Args[0])
	p := e.newTmp()
	fmt.Fprintf(sb, "  %s = inttoptr i64 %s to ptr\n", p, addr.Text)
	if in.Str == "volatile.write" {
		v, _ := e.scalarOperand(sb, f, in.Args[1])
		ty, t := e.volatileWidth(f.Locals[in.Args[1]].Type)
		fmt.Fprintf(sb, "  store volatile %s %s, ptr %s, align %d\n", ty, e.narrowTo(sb, v, ty), p, t)
		e.store(sb, in.Dst, e.irb(sb).Call(rtUnit).Text)
		return true
	}
	dst := f.Locals[in.Dst].Type
	ty, t := e.volatileWidth(dst)
	raw := e.newTmp()
	fmt.Fprintf(sb, "  %s = load volatile %s, ptr %s, align %d\n", raw, ty, p, t)
	e.storeScalarResult(sb, f, in.Dst, e.widenFrom(sb, raw, ty, dst))
	return true
}

func (e *emitter) volatileWidth(t sem.TypeID) (string, int) {
	if t == sem.TyBool {
		return "i8", 1
	}
	if t == sem.TyChar {
		return "i32", 4
	}
	bits := e.r.Types.Width(t)
	return fmt.Sprintf("i%d", bits), bits / 8
}

func (e *emitter) narrowTo(sb *strings.Builder, v Value, ty string) string {
	t := e.newTmp()
	switch {
	case v.Type == TI1:
		fmt.Fprintf(sb, "  %s = zext i1 %s to i8\n", t, v.Text)
	case v.Type == TI32 && ty == "i32":
		return v.Text
	case ty == "i64":
		return v.Text
	default:
		fmt.Fprintf(sb, "  %s = trunc i64 %s to %s\n", t, v.Text, ty)
	}
	return t
}

func (e *emitter) widenFrom(sb *strings.Builder, raw, ty string, dst sem.TypeID) Value {
	t := e.newTmp()
	switch {
	case dst == sem.TyBool:
		fmt.Fprintf(sb, "  %s = trunc i8 %s to i1\n", t, raw)
		return Value{Type: TI1, Text: t}
	case dst == sem.TyChar:
		return Value{Type: TI32, Text: raw}
	case ty == "i64":
		return Value{Type: TI64, Text: raw}
	case e.r.Types.IsSigned(dst):
		fmt.Fprintf(sb, "  %s = sext %s %s to i64\n", t, ty, raw)
	default:
		fmt.Fprintf(sb, "  %s = zext %s %s to i64\n", t, ty, raw)
	}
	return Value{Type: TI64, Text: t}
}
