package llgen

import (
	"fmt"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
)

func (e *emitter) load(sb *strings.Builder, l midir.LocalID) string {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = load ptr, ptr %%l%d\n", t, l)
	return t
}

func (e *emitter) store(sb *strings.Builder, l midir.LocalID, v string) {
	fmt.Fprintf(sb, "  store ptr %s, ptr %%l%d\n", v, l)
}
