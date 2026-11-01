package plan

import midir "kigumi/internal/compiler/mir/ir"

// definesTemp: stores, drops and panics leave Dst without defining it.
func definesTemp(in midir.Inst, isTemp func(midir.LocalID) bool) bool {
	if !isTemp(in.Dst) {
		return false
	}
	switch in.Op {
	case midir.OpNop, midir.OpPanic, midir.OpDrop, midir.OpSetField, midir.OpSetIndex, midir.OpCellSet, midir.OpGlobalSet, midir.OpFutureDone, midir.OpCancelDone:
		return false
	}
	return true
}

func tempDefs(f *midir.Func, isTemp func(midir.LocalID) bool) (map[midir.LocalID]int, map[midir.LocalID]map[int]bool, map[midir.LocalID]int) {
	defs := map[midir.LocalID]int{}
	blocks := map[midir.LocalID]map[int]bool{}
	last := map[midir.LocalID]int{}
	for b, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if !definesTemp(in, isTemp) {
				continue
			}
			defs[in.Dst]++
			last[in.Dst] = b
			if blocks[in.Dst] == nil {
				blocks[in.Dst] = map[int]bool{}
			}
			blocks[in.Dst][b] = true
		}
	}
	return defs, blocks, last
}

// aliasesLocal is true when rewriteCopy leaves the copy a plain alias.
func aliasesLocal(in midir.Inst, defs map[midir.LocalID]int, materialize map[midir.LocalID]bool) bool {
	return !materialize[in.Dst] && (in.Op == midir.OpAlias || !in.Share && defs[in.Dst] <= 1)
}
