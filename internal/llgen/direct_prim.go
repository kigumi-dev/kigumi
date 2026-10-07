package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

var (
	rtDLen          = RuntimeFn{"rt_d_len", []Type{TPtr}, TI64}
	rtDArrayLen     = RuntimeFn{"rt_d_array_len", []Type{TPtr}, TI64}
	rtDBytesGet     = RuntimeFn{"rt_d_bytes_get", []Type{TPtr, TI64, TPtr}, TI32}
	rtDArrayGet     = RuntimeFn{"rt_d_array_get", []Type{TPtr, TI64, TPtr}, TI32}
	rtDCharFromInt  = RuntimeFn{"rt_d_char_from_int", []Type{TI64, TPtr}, TI32}
	rtDParseFloat   = RuntimeFn{"rt_d_parse_float", []Type{TPtr, TPtr}, TI32}
	rtDConvertInt   = RuntimeFn{"rt_d_convert_int", []Type{TI64, TI32, TI32, TI32, TPtr}, TI32}
	rtDConvertFloat = RuntimeFn{"rt_d_convert_float", []Type{TDouble, TI32, TPtr}, TI32}
)

func (e *emitter) directPrimCall(sb *strings.Builder, f *mir.Func, in mir.Inst) {
	args := make([]Value, len(in.Args))
	for i, l := range in.Args {
		args[i] = e.primArg(sb, f, l)
	}
	if e.scalarPrimCall(sb, f, in, args) {
		return
	}
	b := e.irb(sb)
	switch in.DPrim {
	case mir.DPLen:
		e.storeScalarBits(sb, f, in.Dst, b.Call(rtDLen, args[0]).Text)
		return
	case mir.DPArrayLen:
		e.storeScalarBits(sb, f, in.Dst, b.Call(rtDArrayLen, args[0]).Text)
		return
	}
	out := e.stackSlot(sb, "i64")
	var present Value
	switch in.DPrim {
	case mir.DPBytesGet:
		present = b.Call(rtDBytesGet, args[0], args[1], vptr(out))
	case mir.DPArrayGet:
		present = b.Call(rtDArrayGet, args[0], args[1], vptr(out))
	case mir.DPCharFromInt:
		present = b.Call(rtDCharFromInt, args[0], vptr(out))
	case mir.DPParseFloat:
		present = b.Call(rtDParseFloat, args[0], vptr(out))
	case mir.DPConvert:
		present = e.convertRaw(sb, f, in, args[0], out)
	}
	bits := e.newTmp()
	fmt.Fprintf(sb, "  %s = load i64, ptr %s\n", bits, out)
	if _, ok := e.direct[in.Dst]; ok {
		e.storeOptionBits(sb, in.Dst, present.Text, bits)
		return
	}
	e.storeScalarBits(sb, f, in.Dst, bits)
}

// A Char source has numeric kind 0.
func (e *emitter) convertRaw(sb *strings.Builder, f *mir.Func, in mir.Inst, src Value, out string) Value {
	dst := f.Locals[in.Dst].Type
	checked := 0
	if elem, ok := e.r.Types.IsOption(dst); ok {
		dst, checked = elem, 1
	}
	dnk := rtConvertCharTarget
	if dst != sem.TyChar {
		dnk = e.numKind(dst)
	}
	b := e.irb(sb)
	if src.Type == TDouble {
		return b.Call(rtDConvertFloat, src, vi32(dnk), vptr(out))
	}
	snk := 0
	if src.Type == TI32 {
		z := e.newTmp()
		fmt.Fprintf(sb, "  %s = zext i32 %s to i64\n", z, src.Text)
		src = vi64text(z)
	} else {
		snk = e.numKind(f.Locals[in.Args[0]].Type)
	}
	return b.Call(rtDConvertInt, src, vi32(snk), vi32(dnk), vi32(checked), vptr(out))
}

// A Bool widens: an i1 argument leaves the rest of a C int undefined.
func (e *emitter) primArg(sb *strings.Builder, f *mir.Func, l mir.LocalID) Value {
	if _, ok := e.scalarType(f.Locals[l].Type); !ok {
		return vptr(e.load(sb, l))
	}
	v, _ := e.scalarOperand(sb, f, l)
	if v.Type == TI1 {
		t := e.newTmp()
		fmt.Fprintf(sb, "  %s = zext i1 %s to i32\n", t, v.Text)
		return vi32text(t)
	}
	return v
}

func (e *emitter) fromBits(sb *strings.Builder, ty Type, bits string) string {
	t := e.newTmp()
	switch ty {
	case TDouble:
		fmt.Fprintf(sb, "  %s = bitcast i64 %s to double\n", t, bits)
	case TI1:
		fmt.Fprintf(sb, "  %s = icmp ne i64 %s, 0\n", t, bits)
	case TI32:
		fmt.Fprintf(sb, "  %s = trunc i64 %s to i32\n", t, bits)
	default:
		return bits
	}
	return t
}

func (e *emitter) storeScalarBits(sb *strings.Builder, f *mir.Func, dst mir.LocalID, bits string) {
	ty, _ := e.scalarType(f.Locals[dst].Type)
	e.storeScalarResult(sb, f, dst, Value{ty, e.fromBits(sb, ty, bits)})
}

// The payload slot is written for None too, where the runtime left the bits zero.
func (e *emitter) storeOptionBits(sb *strings.Builder, dst mir.LocalID, present, bits string) {
	a := e.direct[dst]
	some, none := 0, 0
	for i, v := range a.Variants {
		if len(v.Payload) == 1 {
			some = i
		} else {
			none = i
		}
	}
	base := fmt.Sprintf("%%l%d", dst)
	has, tag := e.newTmp(), e.newTmp()
	fmt.Fprintf(sb, "  %s = icmp ne i32 %s, 0\n", has, present)
	fmt.Fprintf(sb, "  %s = select i1 %s, i32 %d, i32 %d\n", tag, has, some, none)
	fmt.Fprintf(sb, "  store %s zeroinitializer, ptr %s\n", a.Name, base)
	fmt.Fprintf(sb, "  store %s %s, ptr %s\n", adtTag, tag, e.tagPtr(sb, a, base))
	fld := a.Variants[some].Payload[0]
	fmt.Fprintf(sb, "  store %s %s, ptr %s\n", fld.llType(), e.fromBits(sb, fld.Ty, bits), e.slotPtr(sb, a, base, 0))
}
