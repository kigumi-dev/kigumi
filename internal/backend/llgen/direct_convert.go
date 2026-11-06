package llgen

import (
	"fmt"
	"math"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// convertRaw is the prelude's numeric `toX` on registers: it stores the result bits to out and returns whether there is one.
func (e *emitter) convertRaw(sb *strings.Builder, f *midir.Func, in midir.Inst, src Value, out string) Value {
	dst := f.Locals[in.Dst].Type
	checked := false
	if elem, ok := e.r.Types.IsOption(dst); ok {
		dst, checked = elem, true
	}
	dnk := rtConvertCharTarget
	if dst != sem.TyChar {
		dnk = e.numKind(dst)
	}
	if src.Type == TDouble {
		return e.convertFloat(sb, src, dnk, out)
	}
	if src.Type == TI32 {
		z := e.newTmp()
		fmt.Fprintf(sb, "  %s = zext i32 %s to i64\n", z, src.Text)
		return e.presentBits(sb, "true", z, out)
	}
	return e.convertInt(sb, src, e.numKind(f.Locals[in.Args[0]].Type), dnk, checked, out)
}

// A float source keeps only the range check of i64: the target width is not consulted, as in the VM.
func (e *emitter) convertFloat(sb *strings.Builder, x Value, dnk int, out string) Value {
	if dnk&512 != 0 {
		return e.presentBits(sb, "true", e.bitsOf(sb, x.Text), out)
	}
	lo := e.fcmp(sb, "oge", TDouble, x, Value{TDouble, floatLit(-math.Exp2(63))})
	hi := e.fcmp(sb, "olt", TDouble, x, Value{TDouble, floatLit(math.Exp2(63))})
	ok := e.binOpT(sb, "and", TI1, Value{TI1, lo}, Value{TI1, hi})
	n, bits := e.newTmp(), e.newTmp()
	fmt.Fprintf(sb, "  %s = fptosi double %s to i64\n  %s = select i1 %s, i64 %s, i64 0\n", n, x.Text, bits, ok, n)
	return e.presentBits(sb, ok, bits, out)
}

func (e *emitter) convertInt(sb *strings.Builder, v Value, snk, dnk int, checked bool, out string) Value {
	switch {
	case dnk&rtConvertCharTarget != 0:
		t := e.newTmp()
		fmt.Fprintf(sb, "  %s = and i64 %s, 4294967295\n", t, v.Text)
		return e.presentBits(sb, "true", t, out)
	case dnk&512 != 0:
		conv := "uitofp"
		if isSignedNk(snk) {
			conv = "sitofp"
		}
		d := e.newTmp()
		ty := "double"
		if intBits(dnk) == 32 {
			// One rounding to f32; converting through f64 can cross a midpoint.
			ty = "float"
		}
		fmt.Fprintf(sb, "  %s = %s i64 %s to %s\n", d, conv, v.Text, ty)
		if ty == "float" {
			x := e.newTmp()
			fmt.Fprintf(sb, "  %s = fpext float %s to double\n", x, d)
			d = x
		}
		return e.presentBits(sb, "true", e.bitsOf(sb, d), out)
	case !checked:
		return e.presentBits(sb, "true", e.truncateTo(sb, dnk, v.Text), out)
	}
	ok := e.inRange(sb, v, snk, dnk)
	bits := e.newTmp()
	fmt.Fprintf(sb, "  %s = select i1 %s, i64 %s, i64 0\n", bits, ok, v.Text)
	return e.presentBits(sb, ok, bits, out)
}

func (e *emitter) inRange(sb *strings.Builder, v Value, snk, dnk int) string {
	var conds []string
	if !isSignedNk(snk) || !isSignedNk(dnk) {
		conds = append(conds, e.icmp(sb, "sge", TI64, v, vi64(0)))
	}
	switch {
	case isSignedNk(dnk):
		conds = append(conds, e.icmp(sb, "sge", TI64, v, vi64text(fmt.Sprint(intMin(dnk)))))
		conds = append(conds, e.icmp(sb, "sle", TI64, v, vi64text(fmt.Sprint(intMax(dnk)))))
	case intBits(dnk) < 64:
		conds = append(conds, e.icmp(sb, "ule", TI64, v, vi64text(fmt.Sprint(intMax(dnk)))))
	}
	ok := "true"
	for _, c := range conds {
		ok = e.binOpT(sb, "and", TI1, Value{TI1, ok}, Value{TI1, c})
	}
	return ok
}

func intMax(nk int) uint64 {
	b := intBits(nk)
	if isSignedNk(nk) {
		return uint64(1)<<(b-1) - 1
	}
	if b >= 64 {
		return math.MaxUint64
	}
	return uint64(1)<<b - 1
}

func (e *emitter) charFromInt(sb *strings.Builder, v Value, out string) Value {
	lo := e.icmp(sb, "uge", TI64, v, vi64(0xD800))
	hi := e.icmp(sb, "ule", TI64, v, vi64(0xDFFF))
	surrogate := e.binOpT(sb, "and", TI1, Value{TI1, lo}, Value{TI1, hi})
	scalar := e.icmp(sb, "ule", TI64, v, vi64(0x10FFFF))
	ok := e.binOpT(sb, "and", TI1, Value{TI1, scalar}, Value{TI1, e.xorI1(sb, surrogate)})
	bits := e.newTmp()
	fmt.Fprintf(sb, "  %s = select i1 %s, i64 %s, i64 0\n", bits, ok, v.Text)
	return e.presentBits(sb, ok, bits, out)
}

func (e *emitter) presentBits(sb *strings.Builder, ok, bits, out string) Value {
	t := e.newTmp()
	fmt.Fprintf(sb, "  store i64 %s, ptr %s\n  %s = zext i1 %s to i32\n", bits, out, t, ok)
	return vi32text(t)
}

func (e *emitter) bitsOf(sb *strings.Builder, d string) string {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = bitcast double %s to i64\n", t, d)
	return t
}

// floatLit spells x in the hexadecimal form LLVM reads back exactly.
func floatLit(x float64) string { return fmt.Sprintf("0x%016X", math.Float64bits(x)) }
