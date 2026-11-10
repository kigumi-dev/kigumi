package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/compiler/sem"
)

func (e *emitter) cType(t sem.TypeID) string {
	tt := e.r.Types
	t = e.r.UnderlyingRepr(t)
	switch {
	case t == sem.TyUnit, t == sem.TyNever:
		return "void"
	case tt.IsFloat(t):
		if tt.Width(t) == 32 {
			return "float"
		}
		return "double"
	case tt.IsInteger(t):
		return fmt.Sprintf("i%d", tt.CWidth(t))
	}
	return "ptr"
}

// cArg applies C's variadic default argument promotions: floats widen to
// double, narrow integers to int.
func (e *emitter) cArg(sb *strings.Builder, v string, t sem.TypeID) string {
	tt := e.r.Types
	t = e.r.UnderlyingRepr(t)
	switch {
	case tt.IsFloat(t):
		return "double " + e.unbox(sb, v, t, "double")
	case tt.IsInteger(t) && tt.CWidth(t) > 64:
		return "i128 " + e.unbox(sb, v, t, "i128")
	case tt.IsInteger(t) && tt.CWidth(t) > 32:
		return "i64 " + e.unbox(sb, v, t, "i64")
	case tt.IsInteger(t):
		return "i32 " + e.unbox(sb, v, t, "i32")
	}
	return "ptr " + e.unbox(sb, v, t, "ptr")
}

func (e *emitter) foreignCall(sb *strings.Builder, ent sem.EntityID, args []string, types []sem.TypeID) string {
	sig := e.r.Fn(ent).Sig
	if e.byValue(sig) {
		return e.shimCall(sb, e.byvalShim(ent), sig, nil, args)
	}
	return e.cCall(sb, e.declareForeign(ent), sig, args, types)
}

func (e *emitter) unbox(sb *strings.Builder, v string, t sem.TypeID, ct string) string {
	tt := e.r.Types
	t = e.r.UnderlyingRepr(t)
	switch {
	case ct == "float" || ct == "double":
		d := e.irb(sb).Call(rtFloatVal, vptr(v)).Text
		if ct == "float" {
			out := e.newTmp()
			fmt.Fprintf(sb, "  %s = fptrunc double %s to float\n", out, d)
			return out
		}
		return d
	case tt.Kind(t) == sem.KPtr || tt.IsCFn(t):
		return e.irb(sb).Call(rtPtrVal, vptr(v)).Text
	case ct == "i128":
		return e.unboxScalar(sb, TI128, v).Text
	case tt.IsInteger(t):
		return e.cNarrow(sb, Value{Type: TI64, Text: e.irb(sb).Call(rtIntVal, vptr(v)).Text}, t, ct)
	}
	return e.irb(sb).Call(rtCstr, vptr(v)).Text
}

func (e *emitter) box(sb *strings.Builder, raw string, t sem.TypeID, ct string) string {
	tt := e.r.Types
	t = e.r.UnderlyingRepr(t)
	switch {
	case ct == "float":
		d := e.newTmp()
		fmt.Fprintf(sb, "  %s = fpext float %s to double\n", d, raw)
		return e.irb(sb).Call(rtFloat, vdouble(d), vi32(e.numKind(t))).Text
	case ct == "double":
		return e.irb(sb).Call(rtFloat, vdouble(raw), vi32(e.numKind(t))).Text
	case tt.Kind(t) == sem.KPtr || tt.IsCFn(t):
		return e.irb(sb).Call(rtPtr, vptr(raw)).Text
	case ct == "i128":
		return e.boxNative(sb, scalarInfo{Ty: TI128, Nk: e.numKind(t)}, Value{Type: TI128, Text: raw})
	case tt.IsInteger(t):
		return e.irb(sb).Call(rtInt, e.cWiden(sb, raw, t, ct), vi32(e.numKind(t))).Text
	}
	return e.irb(sb).Call(rtFromCstr, vptr(raw)).Text
}
