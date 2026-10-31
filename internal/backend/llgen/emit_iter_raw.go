package llgen

import (
	"strings"

	"kigumi/internal/compiler/mir"
)

// iterModes is rt_iter_at's mode per builtin, plus one so the zero value means none.
var iterModes = map[string]int{"iter.at": 1, "iter.at_ref": 2, "iter.at_move": 3}

func (e *emitter) iterRaw(f *mir.Func, in mir.Inst) bool {
	if in.Str != "iter.len" && iterModes[in.Str] == 0 || e.isDirect(in.Args[0]) {
		return false
	}
	ty, ok := e.scalarType(f.Locals[in.Dst].Type)
	return ok && ty == TI64
}

// iterBuiltin reads a loop's length and elements with a raw index, so the counter never takes a box.
func (e *emitter) iterBuiltin(sb *strings.Builder, f *mir.Func, in mir.Inst) bool {
	mode := iterModes[in.Str]
	if in.Op != mir.OpBuiltin || in.Str != "iter.len" && mode == 0 || e.isDirect(in.Args[0]) {
		return false
	}
	if in.Str == "iter.len" {
		if !e.isScalar(in.Dst) {
			return false
		}
		e.storeNative(sb, in.Dst, e.irb(sb).Call(rtIterCount, vptr(e.load(sb, in.Args[0]))))
		return true
	}
	it := vptr(e.load(sb, in.Args[0]))
	idx, _ := e.scalarOperand(sb, f, in.Args[1])
	if e.isScalar(in.Dst) {
		e.storeNative(sb, in.Dst, e.irb(sb).Call(rtIterAtInt, it, idx))
		return true
	}
	e.store(sb, in.Dst, e.irb(sb).Call(rtIterAt, it, idx, vi32(mode-1)).Text)
	return true
}
