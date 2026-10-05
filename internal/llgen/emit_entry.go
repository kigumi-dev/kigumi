package llgen

import (
	"fmt"
	"strings"
)

func (e *emitter) mainWrapper() string {
	name := "main"
	if e.freestanding {
		name = "kigumi_main"
	}
	if e.p.Entry == nil {
		return "define i32 @" + name + "(i32 %argc, ptr %argv) {\n  call void @kg_init_immortals()\n  ret i32 0\n}\n"
	}
	entry := e.sym(e.p.Entry)
	var sb strings.Builder
	fmt.Fprintf(&sb, "define i32 @%s(i32 %%argc, ptr %%argv) {\n", name)
	sb.WriteString("  call void @kg_init_immortals()\n")
	e.irb(&sb).Call(rtInit, vi32text("%argc"), vptr("%argv"), vi32(e.typeCount()), vptr("@rt_types"))
	sb.WriteString("  call void @kg_init_statics()\n")
	fmt.Fprintf(&sb, "  %%r = call ptr %s()\n", entry)
	e.irb(&sb).CallAs("%c", rtExitCode, vptr("%r"))
	sb.WriteString("  ret i32 %c\n}\n")
	return sb.String()
}
