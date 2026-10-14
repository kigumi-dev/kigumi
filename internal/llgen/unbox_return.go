package llgen

import "kigumi/internal/mir"

// keepReturnedBoxes leaves a boxed return's value in the box it came from, so returning it costs nothing.
func (e *emitter) keepReturnedBoxes(f *mir.Func, disqualify func(mir.LocalID)) {
	if _, native := e.ssigs[f].ret(); native {
		return
	}
	var work []mir.LocalID
	for _, blk := range f.Blocks {
		if blk.Term.Op == mir.TermReturn && !e.raw[blk.Term.Args[0]] {
			work = append(work, blk.Term.Args[0])
		}
	}
	seen := map[mir.LocalID]bool{}
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
