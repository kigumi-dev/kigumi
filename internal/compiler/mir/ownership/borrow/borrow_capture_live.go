package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

func capturedHolders(p *ownquery.Input, f *midir.Func, place func(midir.LocalID) capturedPlace) map[midir.LocalID][]capturedPlace {
	return holdersFrom(p, f, func(in midir.Inst) []capturedPlace {
		var direct []midir.LocalID
		switch in.Op {
		case midir.OpClosure:
			direct = in.Args
		case midir.OpFutureNew:
			if len(in.Args) > 0 {
				direct = in.Args[1:]
			}
		}
		var out []capturedPlace
		for _, a := range direct {
			out = append(out, place(a))
		}
		return out
	})
}

func holdersFrom(p *ownquery.Input, f *midir.Func, direct func(midir.Inst) []capturedPlace) map[midir.LocalID][]capturedPlace {
	caps := map[midir.LocalID][]capturedPlace{}
	add := func(dst midir.LocalID, cps []capturedPlace) bool {
		grew := false
		for _, cp := range cps {
			if !hasCapturedPlace(caps[dst], cp) {
				caps[dst] = append(caps[dst], cp)
				grew = true
			}
		}
		return grew
	}
	for changed := true; changed; {
		changed = false
		for bi := range f.Blocks {
			for _, in := range f.Blocks[bi].Insts {
				if in.Op == midir.OpDrop {
					continue
				}
				if cps := direct(in); in.Op.Yields() && len(cps) > 0 && add(in.Dst, cps) {
					changed = true
				}
				dst, ok := in.Dst, in.Op.Yields()
				if in.Op == midir.OpSetField || in.Op == midir.OpSetIndex {
					dst, ok = in.Args[0], true
				}
				if !ok || p.R.IsCopy(f.Locals[dst].Type) && p.R.Types.Kind(f.Locals[dst].Type) != sem.KRef {
					continue
				}
				for _, a := range in.Args {
					if cps, ok := caps[a]; ok && a != dst && add(dst, cps) {
						changed = true
					}
				}
			}
		}
	}
	return caps
}

// markDropUse marks a dropped holder live when its drop may run what it holds.
func markDropUse(in midir.Inst, caps map[midir.LocalID][]capturedPlace, live map[midir.LocalID]bool, dropUse func(midir.LocalID) bool) {
	if dropUse != nil && len(in.Args) > 0 && dropUse(in.Args[0]) {
		markCapturedUses(in.Args[:1], caps, live)
	}
}

func hasCapturedPlace(list []capturedPlace, cp capturedPlace) bool {
	for _, x := range list {
		if x.root == cp.root && len(x.path) == len(cp.path) && ownquery.FieldPathsOverlap(x.path, cp.path) {
			return true
		}
	}
	return false
}

// capturedLiveness is each block's live-out holders; a drop is a use only of a holder dropUse names (nil: none).
func capturedLiveness(f *midir.Func, caps map[midir.LocalID][]capturedPlace, dropUse func(midir.LocalID) bool) []map[midir.LocalID]bool {
	n := len(f.Blocks)
	liveOut := make([]map[midir.LocalID]bool, n)
	liveIn := make([]map[midir.LocalID]bool, n)
	for i := range n {
		liveOut[i], liveIn[i] = map[midir.LocalID]bool{}, map[midir.LocalID]bool{}
	}
	for changed := true; changed; {
		changed = false
		for bid := n - 1; bid >= 0; bid-- {
			blk := &f.Blocks[bid]
			out := map[midir.LocalID]bool{}
			for _, t := range blk.Term.Targets {
				for l := range liveIn[t] {
					out[l] = true
				}
			}
			live := map[midir.LocalID]bool{}
			for l := range out {
				live[l] = true
			}
			markCapturedUses(blk.Term.Args, caps, live)
			for i := len(blk.Insts) - 1; i >= 0; i-- {
				in := &blk.Insts[i]
				if in.Op == midir.OpDrop {
					markDropUse(*in, caps, live, dropUse)
					continue
				}
				if in.Op.Yields() {
					delete(live, in.Dst)
				}
				markCapturedUses(in.Args, caps, live)
			}
			if !sameSet(out, liveOut[bid]) || !sameSet(live, liveIn[bid]) {
				changed = true
			}
			liveOut[bid], liveIn[bid] = out, live
		}
	}
	return liveOut
}
