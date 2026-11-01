package build

import (
	"kigumi/internal/compiler/mir"
	opdef "kigumi/internal/compiler/mir/ops"
	mirreach "kigumi/internal/compiler/mir/reach"
)

// resolveStdOps caches each call-shaped instruction's Prim/Prov on Inst, resolved from its Ent once.
func resolveStdOps(p *mir.Program) {
	for _, f := range p.Funcs {
		for bi := range f.Blocks {
			insts := f.Blocks[bi].Insts
			for ii := range insts {
				if in := &insts[ii]; mirreach.NamesStdFn(p.R, in) {
					in.Prim, in.Prov, _ = opdef.ResolveStdOp(p.R, in.Ent)
				}
			}
		}
	}
}
