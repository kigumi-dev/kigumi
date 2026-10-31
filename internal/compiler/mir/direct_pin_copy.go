package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// pinCopyStoreSources keeps the source of `a[i] = b[j]` or `r.f = b.g` boxed when it lives in foreign storage and the destination is written in place.
// an unboxed view of b would make the copy build a box for the slot.
func pinCopyStoreSources(f *midir.Func, out, foreign []bool) {
	defs := map[midir.LocalID]midir.Inst{}
	count := map[midir.LocalID]int{}
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
			if cp.Op != midir.OpShare || !cp.Copy || count[cp.Args[0]] != 1 {
				continue
			}
			ix := defs[cp.Args[0]]
			if ix.Op != midir.OpIndex && ix.Op != midir.OpField || count[ix.Args[0]] != 1 {
				continue
			}
			if arr := ix.Args[0]; viewsForeignField(defs[arr].Op) {
				out[arr], out[cp.Args[0]], out[cp.Dst] = true, true, true
			}
		}
	}
}

func viewsForeignField(op midir.Opcode) bool {
	switch op {
	case midir.OpField, midir.OpFieldMove, midir.OpCellGet, midir.OpUnbox, midir.OpPayload, midir.OpGlobalGet, midir.OpIndex:
		return true
	}
	return false
}
