package plan

import (
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
)

func (p *planner) markDirectPrims(f *midir.Func) {
	for bid := range f.Blocks {
		insts := f.Blocks[bid].Insts
		for i := range insts {
			if insts[i].Op == midir.OpCall {
				insts[i].DPrim = p.DirectPrimOf(f, insts[i])
			}
		}
	}
}

func (p *planner) markPrimResult(f *midir.Func, in midir.Inst, out []bool) {
	if in.Op == midir.OpCall && in.DPrim != opdef.DPNone && p.R.DirectAgg(f.Locals[in.Dst].Type) {
		out[in.Dst] = true
	}
}
