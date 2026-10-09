package llgen

import (
	"fmt"
	"strings"
)

// An artifact with no entry still initializes here: an embedder calls its
// exported functions with no other chance to set the runtime up.
func (e *emitter) mainWrapper() string {
	name := "main"
	switch {
	case e.freestanding:
		name = "kigumi_main"
	case e.entrySymbol != "":
		name = e.entrySymbol
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "define i32 @%s(i32 %%argc, ptr %%argv) {\n", name)
	sb.WriteString("  call void @kg_init_immortals()\n")
	e.irb(&sb).Call(rtInit, vi32text("%argc"), vptr("%argv"), vi32(e.typeCount()), vptr("@rt_types"))
	sb.WriteString("  call void @kg_init_statics()\n")
	if e.p.Entry == nil {
		sb.WriteString("  ret i32 0\n}\n")
		return sb.String()
	}
	fmt.Fprintf(&sb, "  %%r = call ptr %s()\n", e.sym(e.p.Entry))
	e.irb(&sb).CallAs("%c", rtExitCode, vptr("%r"))
	sb.WriteString("  ret i32 %c\n}\n")
	return sb.String()
}
