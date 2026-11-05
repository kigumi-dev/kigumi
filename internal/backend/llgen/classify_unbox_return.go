package llgen

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// keepReturnedBoxes leaves a boxed return's value in the box it came from, so returning it costs nothing.
func (e *emitter) keepReturnedBoxes(f *midir.Func, disqualify func(midir.LocalID)) {
	if _, native := e.ssigs[f].ret(); native {
		return
	}
	var work []midir.LocalID
	for _, blk := range f.Blocks {
		if blk.Term.Op == midir.TermReturn && !e.raw[blk.Term.Args[0]] {
			work = append(work, blk.Term.Args[0])
		}
	}
	seen := map[midir.LocalID]bool{}
	for len(work) > 0 {
		l := work[len(work)-1]
		work = work[:len(work)-1]
		if seen[l] {
			continue
		}
		seen[l] = true
		disqualify(l)
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if in.Dst == l && copyLike(in.Op) && !e.raw[in.Args[0]] {
					work = append(work, in.Args[0])
				}
			}
		}
	}
}

// An alias of a boxed parameter names that box; unboxing it into a register would make a later boxed use allocate a fresh one.
func (e *emitter) keepAliasedBoxes(f *midir.Func, out map[midir.LocalID]scalarInfo, disqualify func(midir.LocalID)) {
	defs := map[midir.LocalID]int{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op.Yields() {
				defs[in.Dst]++
			}
		}
	}
	boxed := map[midir.LocalID]bool{}
	for _, prm := range f.Params {
		if _, reg := out[prm]; !reg && !e.raw[prm] {
			boxed[prm] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if in.Op != midir.OpAlias || defs[in.Dst] != 1 || !boxed[in.Args[0]] || boxed[in.Dst] {
					continue
				}
				disqualify(in.Dst)
				boxed[in.Dst], changed = true, true
			}
		}
	}
}
