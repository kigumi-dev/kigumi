package llgen

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// scalarConst stores OpConst's literal straight into the register instead
// of round-tripping through rt_int/rt_float/rt_bool/rt_char.
func (e *emitter) scalarConst(sb *strings.Builder, in midir.Inst) bool {
	info, ok := e.scalar[in.Dst]
	if !ok {
		return false
	}
	text := scalarConstText(e.r.Types, in)
	if info.Ty == TI128 {
		text = wideConstText(in)
	}
	e.storeNative(sb, in.Dst, Value{info.Ty, text})
	return true
}

func scalarConstText(tt *sem.TypeTable, in midir.Inst) string {
	lit := in.Lit
	switch {
	case in.Str != "" && tt.IsInteger(in.Type):
		return in.Str
	case tt.IsFloat(in.Type):
		bits := 64
		if in.Type == sem.TyF32 {
			bits = 32
		}
		f := literalFloat(lit, bits)
		return fmt.Sprintf("0x%X", math.Float64bits(f))
	case lit.Kind == sem.LitInt:
		if lit.Int == nil {
			return "0"
		}
		if lit.Int.IsInt64() {
			return lit.Int.String()
		}
		return fmt.Sprintf("%d", int64(lit.Int.Uint64()))
	case lit.Kind == sem.LitBool:
		if lit.Bool {
			return "1"
		}
		return "0"
	case lit.Kind == sem.LitChar:
		return strconv.Itoa(int(lit.Char))
	}
	return "0"
}
