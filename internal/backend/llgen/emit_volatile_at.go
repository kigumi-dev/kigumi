package llgen

import (
	"fmt"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

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
	bits := e.r.Types.Width(t)
	return fmt.Sprintf("i%d", bits), bits / 8
}

func (e *emitter) narrowTo(sb *strings.Builder, v Value, ty string) string {
	t := e.newTmp()
	switch {
	case v.Type == TI1:
		fmt.Fprintf(sb, "  %s = zext i1 %s to i8\n", t, v.Text)
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
	case ty == "i64":
		return Value{Type: TI64, Text: raw}
	case e.r.Types.IsSigned(dst):
		fmt.Fprintf(sb, "  %s = sext %s %s to i64\n", t, ty, raw)
	default:
		fmt.Fprintf(sb, "  %s = zext %s %s to i64\n", t, ty, raw)
	}
	return Value{Type: TI64, Text: t}
}
