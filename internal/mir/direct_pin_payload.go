package mir

// detachedPayload reports whether a write through root lands in a payload nobody else reads again: the Result or variant is a call's fresh temporary that only tests and unwraps.
func detachedPayload(f *Func, defs map[LocalID]Inst, count map[LocalID]int, root LocalID) bool {
	l := root
	for {
		def, ok := defs[l]
		if !ok || count[l] != 1 {
			return false
		}
		switch {
		case def.Op == OpAlias || def.Op == OpMove || (def.Op == OpShare && !def.Copy):
			l = def.Args[0]
			continue
		case def.Op != OpPayload:
			return false
		}
		return freshUnwrapped(f, defs, count, def)
	}
}

func freshUnwrapped(f *Func, defs map[LocalID]Inst, count map[LocalID]int, payload Inst) bool {
	holder := payload.Args[0]
	src, ok := defs[holder]
	if !ok || count[holder] != 1 || f.Locals[holder].Ent != 0 || f.Locals[holder].Cell {
		return false
	}
	if src.Op != OpCall && src.Op != OpCallValue {
		return false
	}
	var reads []blockAt
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if !usesArg(in, holder) {
				continue
			}
			switch in.Op {
			case OpIsVariant, OpDrop, OpRelease:
			case OpPayload:
				if in.Index == payload.Index && in.Dst != payload.Dst {
					reads = append(reads, blockAt{BlockID(bid), i})
				}
			default:
				return false
			}
		}
		if blk.Term.Op == TermReturn && usesLocal(blk.Term.Args, holder) {
			return false
		}
	}
	return !anyReachable(f, payload, reads)
}

type blockAt struct {
	blk BlockID
	idx int
}

func usesArg(in Inst, l LocalID) bool { return usesLocal(in.Args, l) }

func usesLocal(args []LocalID, l LocalID) bool {
	for _, a := range args {
		if a == l {
			return true
		}
	}
	return false
}

// A second read of the same slot that can run after the write would see the unwritten copy once the payload is unboxed.
func anyReachable(f *Func, payload Inst, reads []blockAt) bool {
	if len(reads) == 0 {
		return false
	}
	at := locate(f, payload)
	seen := make([]bool, len(f.Blocks))
	stack := []BlockID{at.blk}
	for len(stack) > 0 {
		b := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, r := range reads {
			if r.blk == b && (b != at.blk || r.idx > at.idx) {
				return true
			}
		}
		for _, t := range f.Blocks[b].Term.Targets {
			if !seen[t] {
				seen[t] = true
				stack = append(stack, t)
			}
		}
	}
	for _, r := range reads {
		if r.blk == at.blk && seen[at.blk] {
			return true
		}
	}
	return false
}

func locate(f *Func, payload Inst) blockAt {
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if in.Op == OpPayload && in.Dst == payload.Dst {
				return blockAt{BlockID(bid), i}
			}
		}
	}
	return blockAt{}
}
