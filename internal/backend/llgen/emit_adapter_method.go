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
	slots := paramSlots(midir.BoundTrails(midir.OpClosure), 1, len(f.Params))
	for i := range f.Params {
		v := fmt.Sprintf("%%a%d", i)
		if s := slots[i]; s.env && e.p.ParamLent(f.Ent, i) {
			e.irb(&sb).CallAs(v, rtIterAt, vptr("%env"), vi64(s.index), vi32(1))
		} else {
			e.slotArg(&sb, v, s)
		}
		vals = append(vals, v)
	}
	r := e.callBoxed(&sb, f, vals)
	fmt.Fprintf(&sb, "  ret ptr %s\n}\n\n", r)
	return sb.String()
}
