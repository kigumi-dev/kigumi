package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// flowSets maps a local to the facts a value it holds may carry.
type flowSets map[midir.LocalID]map[uint32]bool

// crossBlockLocals are the locals a fact must be carried across a block edge for: named ones, the task scope, and
// temporaries defined more than once or read outside the block that defines them.
func (c *borrowCheck) crossBlockLocals() map[midir.LocalID]bool {
	keep := map[midir.LocalID]bool{c.spawnScope(): true}
	def := map[midir.LocalID]int{}
	for bid, blk := range c.f.Blocks {
		for _, in := range blk.Insts {
			if !in.Op.Yields() {
				continue
			}
			if b, seen := def[in.Dst]; seen && b != bid || c.f.Locals[in.Dst].Ent != 0 {
				keep[in.Dst] = true
			}
			def[in.Dst] = bid
		}
	}
	for bid, blk := range c.f.Blocks {
		use := func(l midir.LocalID) {
			if b, ok := def[l]; !ok || b != bid {
				keep[l] = true
			}
		}
		for _, in := range blk.Insts {
			for _, a := range in.Args {
				use(a)
			}
		}
		for _, a := range blk.Term.Args {
			use(a)
		}
	}
	return keep
}

func cloneFlowSets(s flowSets) flowSets {
	out := flowSets{}
	for l, m := range s {
		out[l] = make(map[uint32]bool, len(m))
		for f := range m {
			out[l][f] = true
		}
	}
	return out
}

func mergeFlowSets(dst, src flowSets) bool {
	grew := false
	for l, m := range src {
		for f := range m {
			if !dst[l][f] {
				if dst[l] == nil {
					dst[l] = map[uint32]bool{}
				}
				dst[l][f] = true
				grew = true
			}
		}
	}
	return grew
}
