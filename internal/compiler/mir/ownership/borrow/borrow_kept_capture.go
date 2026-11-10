package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// keptCaptures puts what call in passes into each parameter place a closure built here and handed to the call captured
// part of: the call may run the closure, which can store the other operands there.
func (c *borrowCheck) keptCaptures(k *keptCtx, in midir.Inst) {
	// A refs run follows callables through refCallees, and its facts carry no parameter bits.
	if k.refs != nil {
		return
	}
	var from, captured uint64
	for _, a := range in.Args {
		from |= k.facts[a]
		for _, lit := range c.builtClosures(k, a, 8) {
			for _, cp := range lit.Args {
				captured |= k.facts[cp]
			}
		}
	}
	for _, l := range k.g.Params {
		i := k.index(l)
		if i >= 0 && captured&(1<<i|1<<(i+contentShift)) != 0 && (c.lendsPlace(k.g, l) || c.mayHoldPlace(k.g, l)) {
			k.put(l, from)
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
