package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
)

func (e *emitter) load(sb *strings.Builder, l mir.LocalID) string {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = load ptr, ptr %%l%d\n", t, l)
	return t
}

func (e *emitter) store(sb *strings.Builder, l mir.LocalID, v string) {
	fmt.Fprintf(sb, "  store ptr %s, ptr %%l%d\n", v, l)
}
