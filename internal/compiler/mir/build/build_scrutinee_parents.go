package build

import midir "kigumi/internal/compiler/mir/ir"

// projectedFrom lists, nearest first, the temporaries l was projected from that the scope must own before l, so l's count goes
// first and the parent drops its fields in local order; a variant's payload moved into l and a partly defined temporary are skipped.
func (b *builder) projectedFrom(l midir.LocalID, seen map[midir.LocalID]bool, out []midir.LocalID) []midir.LocalID {
	for _, blk := range b.fn.Blocks {
		for _, in := range blk.Insts {
			if !in.Op.Yields() || in.Dst != l || len(in.Args) == 0 {
				continue
			}
			p := in.Args[0]
			if b.fn.Locals[p].Ent != 0 || seen[p] {
				continue
			}
			switch in.Op {
			case midir.OpField, midir.OpIndex, midir.OpPayload:
				if b.definedOnEveryPath(p) {
					out = append(out, p)
				}
			case midir.OpVariant:
			default:
				continue
			}
			seen[p] = true
			out = b.projectedFrom(p, seen, out)
		}
	}
	return out
}

// definedOnEveryPath reports whether every path from the entry to the current block passes a definition of l.
func (b *builder) definedOnEveryPath(l midir.LocalID) bool {
	defs := map[midir.BlockID]bool{}
	for id, blk := range b.fn.Blocks {
		for _, in := range blk.Insts {
			if in.Op.Yields() && in.Dst == l {
				defs[midir.BlockID(id)] = true
			}
		}
	}
	if defs[b.cur] || defs[0] {
		return true
	}
	seen := map[midir.BlockID]bool{0: true}
	for work := []midir.BlockID{0}; len(work) > 0; {
		id := work[len(work)-1]
		work = work[:len(work)-1]
		if id == b.cur {
			return false
		}
		for _, t := range b.fn.Blocks[id].Term.Targets {
			if !seen[t] && !defs[t] {
				seen[t] = true
				work = append(work, t)
			}
		}
	}
	return true
}
