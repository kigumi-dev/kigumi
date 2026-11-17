package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
)

func (c *borrowCheck) keptCaptures(k *keptCtx, in midir.Inst, into func(midir.LocalID, uint64)) {
	var from uint64
	for _, a := range in.Args {
		from |= k.facts[a]
	}
	for _, a := range in.Args {
		for _, lit := range c.builtClosures(k, a, 8) {
			for _, cp := range lit.Args {
				if c.lendsPlace(k.g, cp) || c.mayHoldPlace(k.g, cp) {
					into(cp, from)
				}
			}
		}
	}
}

// builtClosures are the closures and method values k's body builds that a may be or hold, through copies, aggregates
// and calls that hand back only what their arguments bring.
func (c *borrowCheck) builtClosures(k *keptCtx, a midir.LocalID, depth int) []*midir.Inst {
	if depth == 0 {
		return nil
	}
	var out []*midir.Inst
	for _, d := range k.defs[a] {
		switch d.Op {
		case midir.OpClosure:
			out = append(out, d)
		case midir.OpCopy, midir.OpMove, midir.OpAlias, midir.OpRecord, midir.OpVariant, midir.OpArray, midir.OpBox:
		case midir.OpCall:
			if !c.handsBackArgs(*d) {
				continue
			}
		default:
			continue
		}
		for _, x := range d.Args {
			out = append(out, c.builtClosures(k, x, depth-1)...)
		}
	}
	return out
}
