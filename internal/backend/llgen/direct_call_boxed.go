package llgen

import (
	"fmt"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
)

// callBoxed calls f under the boxed convention; a direct signature's crossings live in f's MIR entry.
func (e *emitter) callBoxed(sb *strings.Builder, f *midir.Func, vals []string) string {
	sym := e.sym(f)
	if f.Entry != nil {
		sym = e.useEntry(f)
	} else if e.directSig(f) != nil {
		panic("llgen: " + f.Name + " has a direct signature and no boxed entry")
	} else if ss := e.ssigs[f]; ss != nil {
		return e.callNativeBoxed(sb, f, ss, vals)
	}
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = call ptr %s(%s)\n", t, sym, joinArgs(vals))
	return t
}
