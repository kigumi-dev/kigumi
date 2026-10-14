package mir

// definesTemp: stores, drops and panics leave Dst without defining it.
func definesTemp(in Inst, isTemp func(LocalID) bool) bool {
	if !isTemp(in.Dst) {
		return false
	}
	switch in.Op {
	case OpNop, OpPanic, OpDrop, OpSetField, OpSetIndex, OpCellSet, OpGlobalSet, OpFutureDone:
		return false
	}
	return true
}

func tempDefs(f *Func, isTemp func(LocalID) bool) (map[LocalID]int, map[LocalID]map[int]bool, map[LocalID]int) {
	defs := map[LocalID]int{}
	blocks := map[LocalID]map[int]bool{}
	last := map[LocalID]int{}
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
func aliasesLocal(in Inst, defs map[LocalID]int, materialize map[LocalID]bool) bool {
	return !materialize[in.Dst] && (in.Op == OpAlias || !in.Share && defs[in.Dst] <= 1)
}
