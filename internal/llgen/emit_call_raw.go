package llgen

import (
	"fmt"
	"strings"
)

func (e *emitter) call(sb *strings.Builder, fn string, args []string) string {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = call ptr @%s(%s)\n", t, fn, joinArgs(args))
	return t
}

func (e *emitter) callVoid(sb *strings.Builder, fn string, args []string) {
	fmt.Fprintf(sb, "  call void @%s(%s)\n", fn, joinArgs(args))
}

func joinArgs(args []string) string {
	var parts []string
	for _, a := range args {
		if strings.HasPrefix(a, "%") || strings.HasPrefix(a, "@") || strings.HasPrefix(a, "null") {
			parts = append(parts, "ptr "+a)
		} else {
			parts = append(parts, a)
		}
	}
	return strings.Join(parts, ", ")
}
