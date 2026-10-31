package ir

import opdef "kigumi/internal/compiler/mir/ops"

// StdOp reports the native op id a resolved Inst carries, if any.
func (in *Inst) StdOp() (opdef.StdOpId, bool) {
	if in.Prim != 0 {
		return opdef.StdOpId(in.Prim - 1), true
	}
	if in.Prov != 0 {
		return opdef.StdOpId(in.Prov - 1), true
	}
	return 0, false
}
