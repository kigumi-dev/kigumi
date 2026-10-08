package mir

// pinned marks locals that keep the boxed form: reference and cell objects, and roots a view writes through.
func (p *Program) pinned(f *Func) []bool {
	out := make([]bool, len(f.Locals))
	for i := range f.Locals {
		out[i] = f.Locals[i].Cell
	}
	foreign := foreignLocals(f)
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			switch in.Op {
			case OpBorrow, OpNewCell:
				// Both yield an object over the argument's storage, and
				// OpNewCell's result keeps the content's type while holding a cell.
				out[in.Args[0]], out[in.Dst] = true, true
			case OpSetField, OpSetIndex:
				if foreign[in.Args[0]] {
					out[in.Args[0]] = true
				}
			}
		}
	}
	spreadPins(f, out)
	return out
}

func spreadPins(f *Func, out []bool) {
	for changed := true; changed; {
		changed = false
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if !in.Op.Yields() || !out[in.Dst] {
					continue
				}
				if src, ok := storageSource(in); ok && !out[src] {
					out[src], changed = true, true
				}
			}
		}
	}
}

// storageSource names the local a result shares storage with, so a pin on
// the view reaches the value it was taken from.
func storageSource(in Inst) (LocalID, bool) {
	switch in.Op {
	case OpCopy, OpAlias, OpMove, OpField, OpFieldMove, OpPayload, OpIndex:
		return in.Args[0], true
	case OpShare:
		return in.Args[0], !in.Copy
	}
	return 0, false
}
