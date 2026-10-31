package mir

// BoxConsumed reports that in takes over argument i of a box a crossing made; otherwise the call site releases it.
func (p *Program) BoxConsumed(in Inst, i int) bool {
	switch in.Op {
	case OpRecord, OpVariant, OpArray, OpBox, OpGlobalSet, OpNewCell, OpDrop, OpRelease, OpAwait, OpAlias, OpMove:
		return true
	case OpSetField, OpCellSet:
		return i == 1
	case OpSetIndex:
		return i == 2
	case OpCallValue:
		return i > 0
	case OpCall:
		return !p.ParamLent(in.Ent, i)
	}
	return false
}

// heldBoxes finds the arguments that are a crossing's box still owed a release by the call site after in.
func (p *Program) heldBoxes(f *Func) func(in *Inst, i int) bool {
	boxes := map[LocalID]bool{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == OpBoxDirect && f.Locals[in.Dst].Cross {
				boxes[in.Dst] = true
			}
		}
	}
	if len(boxes) == 0 {
		return nil
	}
	return func(in *Inst, i int) bool {
		return in.Op != OpBoxDirect && in.Op != OpUnboxDirect && boxes[in.Args[i]] && !p.BoxConsumed(*in, i)
	}
}

// slotHeldBoxes gives an async body's held boxes a frame slot, so a panic in the call sweeps them like any temporary.
func (p *Program) slotHeldBoxes(f *Func) {
	held := p.heldBoxes(f)
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
