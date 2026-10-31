package llgen

import (
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
)

// builtinCall lowers an OpBuiltin; `task.close` needs the running body's own Future, which only an async body has.
func (e *emitter) builtinCall(sb *strings.Builder, in midir.Inst, args []string) string {
	if in.Str == "task.close" {
		return e.irb(sb).Call(rtTaskClose, vptr("%fut")).Text
	}
	arr := e.argArray(sb, args)
	return e.irb(sb).Call(rtBuiltin, vptr(e.cstr(in.Str)), vi32(len(args)), vptr(arr)).Text
}
