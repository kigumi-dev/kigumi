package mir

// pinCopyStoreSources keeps the array behind `a[i] = b[j]` boxed when b lives in foreign storage and a's slot is written in place:
// an unboxed view of b would make the copy build a box for the slot.
func pinCopyStoreSources(f *Func, out, foreign []bool) {
	defs := map[LocalID]Inst{}
	count := map[LocalID]int{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op.Yields() {
				defs[in.Dst] = in
				count[in.Dst]++
			}
		}
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op != OpSetIndex || !foreign[in.Args[0]] || count[in.Args[2]] != 1 {
				continue
			}
			cp := defs[in.Args[2]]
			if cp.Op != OpShare || !cp.Copy || count[cp.Args[0]] != 1 {
				continue
			}
			ix := defs[cp.Args[0]]
			if ix.Op != OpIndex || count[ix.Args[0]] != 1 {
				continue
			}
			if arr := ix.Args[0]; viewsForeignField(defs[arr].Op) {
				out[arr], out[cp.Args[0]], out[cp.Dst] = true, true, true
			}
		}
	}
}

func viewsForeignField(op Opcode) bool {
	switch op {
	case OpField, OpFieldMove, OpCellGet, OpUnbox, OpPayload, OpGlobalGet, OpIndex:
		return true
	}
	return false
}
