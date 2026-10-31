package llgen

import (
	"strings"

	"kigumi/internal/compiler/mir"
)

func (e *emitter) bindValue(sb *strings.Builder, in mir.Inst, args []string) string {
	fn, own := e.p.ByEnt[in.Ent]
	if !own && len(args) == 0 {
		return e.stdBindSlot(e.stdKey(in.Ent), mir.LentMask(e.r, in.Ent))
	}
	arr := e.argArray(sb, args)
	switch {
	case own && in.Async:
		return e.irb(sb).Call(rtClosureBindAsync, vptr(e.asyncDescSym(fn)), vi32(len(args)), vptr(arr)).Text
	case own:
		return e.markLent(sb, in, 0, len(args) == 0, e.closureCtor(sb, e.useBind(fn), false, args, arr))
	}
	return e.irb(sb).Call(rtStdBind, vptr(e.stdKeyStr(e.stdKey(in.Ent))), vi32(mir.LentMask(e.r, in.Ent)), vi32(len(args)), vptr(arr)).Text
}
