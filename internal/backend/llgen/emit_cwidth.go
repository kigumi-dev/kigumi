package llgen

import (
	"cmp"
	"fmt"
	"strings"

	"kigumi/internal/compiler/sem"
)

// checkCRange panics unless the 64-bit usize or isize value w fits ct, the
// narrower size_t or ptrdiff_t of a 32-bit target.
func (e *emitter) checkCRange(sb *strings.Builder, w string, t sem.TypeID, ct string) {
	ext, name, msg := "zext", "size_t", "usize value does not fit the %s of this target"
	if t == sem.TyIsize {
		ext, name = "sext", "ptrdiff_t"
		msg = "isize value does not fit the %s of this target"
	}
	narrow, wide, bad := e.newTmp(), e.newTmp(), e.newTmp()
	fmt.Fprintf(sb, "  %s = trunc i64 %s to %s\n", narrow, w, ct)
	fmt.Fprintf(sb, "  %s = %s %s %s to i64\n", wide, ext, ct, narrow)
	fmt.Fprintf(sb, "  %s = icmp ne i64 %s, %s\n", bad, wide, w)
	e.overflowTrap(sb, bad, fmt.Sprintf(msg, name))
}

// cWiden turns the C value raw of type t into its register form.
func (e *emitter) cWiden(sb *strings.Builder, raw string, t sem.TypeID, ct string) Value {
	tt := e.r.Types
	t = e.r.UnderlyingRepr(t)
	switch {
	case ct == "float":
		return Value{Type: TFloat, Text: raw}
	case ct == "double":
		return Value{Type: TDouble, Text: raw}
	case tt.Kind(t) == sem.KPtr:
		return Value{Type: TCPtr, Text: raw}
	case ct == "i64":
		return Value{Type: TI64, Text: raw}
	case ct == "i128":
		return Value{Type: TI128, Text: raw}
	}
	w, ext := e.newTmp(), "sext"
	if !tt.IsSigned(t) {
		ext = "zext"
	}
	fmt.Fprintf(sb, "  %s = %s %s %s to i64\n", w, ext, ct, raw)
	return Value{Type: TI64, Text: w}
}

// cNarrow is cWiden's inverse: register value v of type t as C type ct.
func (e *emitter) cNarrow(sb *strings.Builder, v Value, t sem.TypeID, ct string) string {
	t = e.r.UnderlyingRepr(t)
	if v.Type != TI64 || ct == "i64" {
		return v.Text
	}
	if t == sem.TyUsize || t == sem.TyIsize {
		e.checkCRange(sb, v.Text, t, ct)
	}
	out := e.newTmp()
	fmt.Fprintf(sb, "  %s = trunc i64 %s to %s\n", out, v.Text, ct)
	return out
}

// cRegister reports a C argument or result type that stays in registers: a
// number (an i128 or u128 takes a register pair), a float or a pointer.
func (e *emitter) cRegister(t sem.TypeID) bool {
	tt := e.r.Types
	t = e.r.UnderlyingRepr(t)
	return tt.IsFloat(t) || tt.Kind(t) == sem.KPtr || tt.IsInteger(t)
}

// cIntABI is which C integers the target's ABI extends to a full register, as clang marks them signext or zeroext.
type cIntABI struct{ narrow, word bool }

// The 64-bit Arm ABI outside Apple and the Windows x64 ABI leave narrow integers unextended; RV64 sign-extends 32-bit ones too.
func cIntABIFor(arch, os string) cIntABI {
	return cIntABI{narrow: !(arch == "arm64" && os != "darwin") && !(cmp.Or(arch, "amd64") == "amd64" && os == "windows"), word: arch == "riscv64"}
}

// cExt is the extension attribute of C integer type t on this target: RV64 makes u32 signext as well.
func (e *emitter) cExt(t sem.TypeID) string {
	tt := e.r.Types
	t = e.r.UnderlyingRepr(t)
	if !tt.IsInteger(t) {
		return ""
	}
	switch w := tt.CWidth(t); {
	case w > 32, w == 32 && !e.cabi.word, w < 32 && !e.cabi.narrow:
		return ""
	case w == 32 || tt.IsSigned(t):
		return " signext"
	}
	return " zeroext"
}
