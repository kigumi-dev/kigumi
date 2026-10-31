package llgen

import (
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
)

// emitPanic leaves a panic marked NoTrap out entirely: the block's unreachable terminator is all that remains.
func (e *emitter) emitPanic(sb *strings.Builder, in midir.Inst) {
	if !in.NoTrap {
		e.irb(sb).Call(rtPanic, vptr(e.cstr(in.Str)))
	}
}

// scalarBinaryOp is nativeBinaryOp, except that a guarded `+` becomes a plain add with no overflow check.
func (e *emitter) scalarBinaryOp(sb *strings.Builder, in midir.Inst, ty Type, nk int, l, r Value) (Value, bool) {
	if in.NoTrap && in.Str == "+" && (ty == TI64 || ty == TI128) {
		return Value{ty, e.binOpT(sb, "add", ty, l, r)}, true
	}
	return e.nativeBinaryOp(sb, in.Str, ty, nk, l, r)
}
