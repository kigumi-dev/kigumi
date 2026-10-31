package mir

// pinCopyStoreSources keeps the source of `a[i] = b[j]` or `r.f = b.g` boxed when it lives in foreign storage and the destination is written in place.
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
			v, ok := storedValue(in)
			if !ok || !foreign[in.Args[0]] || count[v] != 1 {
				continue
			}
			cp := defs[v]
			if cp.Op != OpShare || !cp.Copy || count[cp.Args[0]] != 1 {
				continue
			}
			ix := defs[cp.Args[0]]
			if ix.Op != OpIndex && ix.Op != OpField || count[ix.Args[0]] != 1 {
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
