package plan

import midir "kigumi/internal/compiler/mir/ir"

// slotHeldBoxes gives an async body's held boxes a frame slot, so a panic in the call sweeps them like any temporary.
func (p *planner) slotHeldBoxes(f *midir.Func) {
	held := p.HeldBoxes(f)
	if held == nil {
		return
	}
	for _, blk := range f.Blocks {
		for k := range blk.Insts {
			in := &blk.Insts[k]
			for i, a := range in.Args {
				if held(in, i) {
					f.Slot[a] = 0
				}
			}
		}
	}
}
