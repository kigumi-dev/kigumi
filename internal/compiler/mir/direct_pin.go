package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// pinned marks locals that keep the boxed form: reference and cell objects, and roots a view writes through.
func (p *Program) pinned(f *midir.Func) []bool {
	out := make([]bool, len(f.Locals))
	for i := range f.Locals {
		out[i] = f.Locals[i].Cell
	}
	foreign := foreignLocals(f)
	defs, count := defCounts(f)
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			switch in.Op {
			case midir.OpBorrow, midir.OpNewCell:
				// Both yield an object over the argument's storage, and
				// OpNewCell's result keeps the content's type while holding a cell.
				out[in.Args[0]], out[in.Dst] = true, true
			case midir.OpSetField, midir.OpSetIndex:
				if foreign[in.Args[0]] && !detachedPayload(f, defs, count, in.Args[0]) {
					out[in.Args[0]] = true
				}
			}
		}
	}
	pinCopyStoreSources(f, out, foreign)
	p.spreadPins(f, out)
	return out
}

// A counted payload is a reference the slot shares with the view, so a pin on the view leaves the Option or Result unboxed.
func (p *Program) spreadPins(f *midir.Func, out []bool) {
	for changed := true; changed; {
		changed = false
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if !in.Op.Yields() || !out[in.Dst] || in.Op == midir.OpPayload && p.R.CountedSlot(f.Locals[in.Dst].Type) {
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
func storageSource(in midir.Inst) (midir.LocalID, bool) {
	switch in.Op {
	case midir.OpCopy, midir.OpAlias, midir.OpMove, midir.OpField, midir.OpFieldMove, midir.OpPayload, midir.OpIndex:
		return in.Args[0], true
	case midir.OpShare:
		return in.Args[0], !in.Copy
	}
	return 0, false
}

func defCounts(f *midir.Func) (map[midir.LocalID]midir.Inst, map[midir.LocalID]int) {
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
	return defs, count
}
