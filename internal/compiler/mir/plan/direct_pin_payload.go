package plan

import midir "kigumi/internal/compiler/mir/ir"

// detachedPayload reports whether a write through root lands in a payload nobody else reads again: the Result or variant is a call's fresh temporary that only tests and unwraps.
func detachedPayload(f *midir.Func, defs map[midir.LocalID]midir.Inst, count map[midir.LocalID]int, root midir.LocalID) bool {
	l := root
	for {
		def, ok := defs[l]
		if !ok || count[l] != 1 {
			return false
		}
		switch {
		case def.Op == midir.OpAlias || def.Op == midir.OpMove || (def.Op == midir.OpShare && !def.Copy):
			l = def.Args[0]
			continue
		case def.Op != midir.OpPayload:
			return false
		}
		return freshUnwrapped(f, defs, count, def)
	}
}

func freshUnwrapped(f *midir.Func, defs map[midir.LocalID]midir.Inst, count map[midir.LocalID]int, payload midir.Inst) bool {
	holder := payload.Args[0]
	src, ok := defs[holder]
	if !ok || count[holder] != 1 || f.Locals[holder].Ent != 0 || f.Locals[holder].Cell {
		return false
	}
	if src.Op != midir.OpCall && src.Op != midir.OpCallValue {
		return false
	}
	var reads []blockAt
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if !usesArg(in, holder) {
				continue
			}
			switch in.Op {
			case midir.OpIsVariant, midir.OpDrop, midir.OpRelease:
			case midir.OpPayload:
				if in.Index == payload.Index && in.Dst != payload.Dst {
					reads = append(reads, blockAt{midir.BlockID(bid), i})
				}
			default:
				return false
			}
		}
		if blk.Term.Op == midir.TermReturn && usesLocal(blk.Term.Args, holder) {
			return false
		}
	}
	return !anyReachable(f, payload, reads)
}

type blockAt struct {
	blk midir.BlockID
	idx int
}

func usesArg(in midir.Inst, l midir.LocalID) bool { return usesLocal(in.Args, l) }

func usesLocal(args []midir.LocalID, l midir.LocalID) bool {
	for _, a := range args {
		if a == l {
			return true
		}
	}
	return false
}

// A second read of the same slot that can run after the write would see the unwritten copy once the payload is unboxed.
func anyReachable(f *midir.Func, payload midir.Inst, reads []blockAt) bool {
	if len(reads) == 0 {
		return false
	}
	at := locate(f, payload)
	seen := make([]bool, len(f.Blocks))
	stack := []midir.BlockID{at.blk}
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

func locate(f *midir.Func, payload midir.Inst) blockAt {
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if in.Op == midir.OpPayload && in.Dst == payload.Dst {
				return blockAt{midir.BlockID(bid), i}
			}
		}
	}
	return blockAt{}
}
