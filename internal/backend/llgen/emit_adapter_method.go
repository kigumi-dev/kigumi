package llgen

import (
	"fmt"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
)

func (e *emitter) methodAdapterSym(f *midir.Func) string {
	return "@\"" + e.symName(f) + "$methodvalue\""
}

func (e *emitter) methodAdapter(f *midir.Func) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "define ptr %s(ptr %%env, ptr %%args) {\n", e.methodAdapterSym(f))
	var vals []string
	for i := range f.Params {
		v := fmt.Sprintf("%%a%d", i)
		if i == 0 && e.p.ParamLent(f.Ent, 0) {
			e.irb(&sb).CallAs(v, rtIterAt, vptr("%env"), vi64(0), vi32(1))
		} else if i == 0 {
			e.irb(&sb).CallAs(v, rtAt, vptr("%env"), vi64(0))
		} else {
			e.irb(&sb).CallAs(v, rtAt, vptr("%args"), vi64(i-1))
		}
		vals = append(vals, v)
	}
	r := e.callBoxed(&sb, f, vals)
	fmt.Fprintf(&sb, "  ret ptr %s\n}\n\n", r)
	return sb.String()
}
