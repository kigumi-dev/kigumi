package ir

import (
	"fmt"
	"strings"
)

// RenderExterns renders the `declare` list for fns, one line per entry in
// table order.
func RenderExterns(fns []RuntimeFn) string {
	var sb strings.Builder
	for _, fn := range fns {
		params := make([]string, len(fn.Params))
		for i, p := range fn.Params {
			params[i] = p.String()
		}
		fmt.Fprintf(&sb, "declare %s @%s(%s)\n", fn.Ret, fn.Name, strings.Join(params, ", "))
	}
	sb.WriteString("\n")
	return sb.String()
}
