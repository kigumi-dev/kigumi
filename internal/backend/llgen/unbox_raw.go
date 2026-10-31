package llgen

import (
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
)

// rawScalars are the locals whose value no box holds yet: boxing one costs an allocation wherever it
// happens, so a reader that never keeps it can format it from the frame instead.
func (e *emitter) rawScalars(f *midir.Func) map[midir.LocalID]bool {
	out := map[midir.LocalID]bool{}
	for i, prm := range f.Params {
		if _, ok := e.ssigs[f].param(i); ok {
			out[prm] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if !in.Op.Yields() || out[in.Dst] || !e.rawDef(f, in, out) {
					continue
				}
				out[in.Dst], changed = true, true
			}
		}
	}
	return out
}

func (e *emitter) rawDef(f *midir.Func, in midir.Inst, raw map[midir.LocalID]bool) bool {
	switch in.Op {
	case midir.OpPayload, midir.OpField, midir.OpFieldMove, midir.OpIndex:
		return e.isDirect(in.Args[0])
	case midir.OpBinary, midir.OpUnary:
		return true
	case midir.OpBuiltin:
		return in.Str == "direct.hash" || in.Str == "direct.eq"
	case midir.OpCallValue:
		return e.callvI64(f, in)
	case midir.OpCall:
		_, native := e.calleeScalarSig(in).ret()
		return in.DPrim != opdef.DPNone || native || e.wideCallOp(f, in) != "" || e.rawConvert(f, in) || e.cNative(in) && e.cDstInReg(f, in)
	case midir.OpAlias, midir.OpMove, midir.OpShare:
		return raw[in.Args[0]]
	}
	return false
}

func (e *emitter) callvI64(f *midir.Func, in midir.Inst) bool {
	if _, ok := e.direct[in.Args[0]]; ok {
		return false
	}
	ty, ok := e.scalarType(f.Locals[in.Dst].Type)
	return ok && ty == TI64
}
