package llgen

import "kigumi/internal/mir"

// rawScalars are the locals whose value no box holds yet: boxing one costs an allocation wherever it
// happens, so a reader that never keeps it can format it from the frame instead.
func (e *emitter) rawScalars(f *mir.Func) map[mir.LocalID]bool {
	out := map[mir.LocalID]bool{}
	for i, prm := range f.Params {
		if _, ok := e.ssigs[f].param(i); ok {
			out[prm] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if !in.Op.Yields() || out[in.Dst] || !e.rawDef(in, out) {
					continue
				}
				out[in.Dst], changed = true, true
			}
		}
	}
	return out
}

func (e *emitter) rawDef(in mir.Inst, raw map[mir.LocalID]bool) bool {
	switch in.Op {
	case mir.OpPayload, mir.OpField, mir.OpFieldMove, mir.OpIndex:
		return e.isDirect(in.Args[0])
	case mir.OpBinary, mir.OpUnary:
		return true
	case mir.OpCall:
		_, native := e.calleeScalarSig(in).ret()
		return in.DPrim != mir.DPNone || native
	case mir.OpAlias, mir.OpMove, mir.OpShare:
		return raw[in.Args[0]]
	}
	return false
}
