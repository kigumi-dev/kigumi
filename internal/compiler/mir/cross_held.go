package mir

import midir "kigumi/internal/compiler/mir/ir"

// BoxConsumed reports that in takes over argument i of a box a crossing made; otherwise the call site releases it.
func (p *Program) BoxConsumed(in midir.Inst, i int) bool {
	switch in.Op {
	case midir.OpRecord, midir.OpVariant, midir.OpArray, midir.OpBox, midir.OpGlobalSet, midir.OpNewCell, midir.OpDrop, midir.OpRelease, midir.OpAwait, midir.OpAlias, midir.OpMove:
		return true
	case midir.OpSetField, midir.OpCellSet:
		return i == 1
	case midir.OpSetIndex:
		return i == 2
	case midir.OpCallValue:
		return i > 0
	case midir.OpCall:
		return !p.ParamLent(in.Ent, i)
	}
	return false
}

// HeldBoxes finds the arguments that are a crossing's box still owed a release by the call site after in.
func (p *Program) HeldBoxes(f *midir.Func) func(in *midir.Inst, i int) bool {
	boxes := map[midir.LocalID]bool{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpBoxDirect && f.Locals[in.Dst].Cross {
				boxes[in.Dst] = true
			}
		}
	}
	if len(boxes) == 0 {
		return nil
	}
	return func(in *midir.Inst, i int) bool {
		return in.Op != midir.OpBoxDirect && in.Op != midir.OpUnboxDirect && boxes[in.Args[i]] && !p.BoxConsumed(*in, i)
	}
}
