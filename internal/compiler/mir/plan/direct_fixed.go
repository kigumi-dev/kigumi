package plan

import midir "kigumi/internal/compiler/mir/ir"

func (p *planner) markFixed(f *midir.Func) {
	fixed := p.FixedLocals(f)
	for bid := range f.Blocks {
		insts := f.Blocks[bid].Insts
		for i := range insts {
			if insts[i].Op == midir.OpBoxDirect && fixed[insts[i].Args[0]] {
				insts[i].Fixed = true
			}
		}
	}
}
