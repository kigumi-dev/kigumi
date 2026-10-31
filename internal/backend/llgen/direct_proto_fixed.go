package llgen

import (
	"fmt"
	"strings"
)

// The elements fold in order from the same basis a record's fields do.
func (e *emitter) protoFixed(sb *strings.Builder, a *aggInfo, hash bool) {
	if hash {
		sb.WriteString("  %h = alloca i64\n")
		fmt.Fprintf(sb, "  store i64 %s, ptr %%h\n", offsetBasis)
		e.countLoop(sb, a.Len, func(j string) {
			x, m, cur := e.newTmp(), e.newTmp(), e.newTmp()
			fmt.Fprintf(sb, "  %s = load i64, ptr %%h\n  %s = xor i64 %s, %s\n  %s = mul i64 %s, %s\n  store i64 %s, ptr %%h\n",
				cur, x, cur, e.leafHash(sb, *a.Arr, e.elemAt(sb, a, "%a", j)), m, x, fnvPrimeText, m)
		})
		sb.WriteString("  %r = load i64, ptr %h\n  ret i64 %r\n")
		return
	}
	e.countLoop(sb, a.Len, func(j string) {
		next := e.label("same")
		same := e.leafEq(sb, *a.Arr, e.elemAt(sb, a, "%a", j), e.elemAt(sb, a, "%b", j))
		fmt.Fprintf(sb, "  br i1 %s, label %%%s, label %%no\n%s:\n", same, next, next)
	})
	sb.WriteString("  ret i1 true\nno:\n  ret i1 false\n")
}
