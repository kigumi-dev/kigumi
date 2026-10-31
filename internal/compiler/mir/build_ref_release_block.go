package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	borrowpass "kigumi/internal/compiler/mir/ownership/borrow"
)

// hoistRefDrops moves the scope-end releases of l to just after the last use of l or of a borrow read from it on each
// path, else each into the earliest block that runs exactly when its own block does, so a later write to l's owner is free.
func (g *ghostSet) hoistRefDrops(p *ownquery.Input, f *midir.Func, l midir.LocalID, flow *blockFlow) {
	defs, home := 0, -1
	var drops []int
	for bid, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op.Yields() && in.Dst == l {
				defs, home = defs+1, bid
			}
			if releasesRef(in, l) {
				drops = append(drops, bid)
			}
		}
	}
	held := refDerived(p, f, l)
	start, end := g.ghostRange(f)
	ghostUse := slices.ContainsFunc(f.Blocks[start:end], func(b midir.Block) bool {
		return slices.ContainsFunc(b.Insts, func(in midir.Inst) bool { return touchesHeld(in, l, held) })
	})
	if defs != 1 || ghostUse || slices.ContainsFunc(f.Blocks, func(b midir.Block) bool {
		return slices.ContainsFunc(b.Term.Args, func(a midir.LocalID) bool { return held[a] })
	}) {
		return
	}
	live := heldLiveIn(f, l, held)
	if g.releaseAtExits(f, l, held, live, home, flow) {
		return
	}
	for _, d := range drops {
		i := slices.IndexFunc(f.Blocks[d].Insts, func(in midir.Inst) bool { return releasesRef(in, l) })
		to, at := d, lastTouch(f.Blocks[d].Insts[:i], l, held)
		for j := d; at == 0 && !live[j] && j != home; {
			j = flow.idom(j)
			if j < 0 || !flow.postDominates(d, j) || flow.liveOut(j, live) {
				break
			}
			to, at = j, lastTouch(f.Blocks[j].Insts, l, held)
		}
		g.insertInst(f, to, at, g.deleteInst(f, d, i))
	}
}

// lastTouch is the index just past the last instruction of insts that defines l or reads l or a borrow from it.
func lastTouch(insts []midir.Inst, l midir.LocalID, held map[midir.LocalID]bool) int {
	for i := len(insts) - 1; i >= 0; i-- {
		if touchesHeld(insts[i], l, held) {
			return i + 1
		}
	}
	return 0
}

// callResult reports whether l's one definition copies a call's result.
func callResult(defs map[midir.LocalID][]midir.Inst, l midir.LocalID) bool {
	for range len(defs) + 1 {
		if len(defs[l]) != 1 {
			return false
		}
		switch d := defs[l][0]; d.Op {
		case midir.OpCall, midir.OpCallValue:
			return true
		case midir.OpCopy, midir.OpMove, midir.OpAlias:
			l = d.Args[0]
		default:
			return false
		}
	}
	return false
}

func releasesRef(in midir.Inst, l midir.LocalID) bool {
	return in.Op == midir.OpDrop && in.EndScope && len(in.Args) == 1 && in.Args[0] == l
}

// touchesHeld reports a use of l or of a borrow read from it; a scope-end release of one is no use.
func touchesHeld(in midir.Inst, l midir.LocalID, held map[midir.LocalID]bool) bool {
	if in.Op == midir.OpDrop && in.EndScope && len(in.Args) == 1 && held[in.Args[0]] {
		return false
	}
	return in.Dst == l && in.Op.Yields() || slices.ContainsFunc(in.Args, func(a midir.LocalID) bool { return held[a] })
}

// refDerived is l with what is read from it that may keep pointing into its referent: a borrow-carrying
// value, or a temporary place a further projection or borrow starts from.
func refDerived(p *ownquery.Input, f *midir.Func, l midir.LocalID) map[midir.LocalID]bool {
	held := map[midir.LocalID]bool{l: true}
	bc := borrowpass.NewTypeFacts(p.R)
	for grew := true; grew; {
		grew = false
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if !in.Op.Yields() || held[in.Dst] || !slices.ContainsFunc(in.Args, func(a midir.LocalID) bool { return held[a] }) {
					continue
				}
				t := f.Locals[in.Dst].Type
				place := f.Locals[in.Dst].Ent == 0 && placeStep(in.Op)
				if bc.HoldsBorrow(t) || bc.NamesTypeParam(t) || place && !p.R.IsCopy(t) {
					held[in.Dst], grew = true, true
				}
			}
		}
	}
	return held
}

func placeStep(op midir.Opcode) bool {
	switch op {
	case midir.OpField, midir.OpPayload, midir.OpIndex, midir.OpCellGet, midir.OpBorrow, midir.OpCopy, midir.OpMove, midir.OpAlias:
		return true
	}
	return false
}

// heldLiveIn marks the blocks some use of held is reachable from without passing l's definition.
func heldLiveIn(f *midir.Func, l midir.LocalID, held map[midir.LocalID]bool) []bool {
	n := len(f.Blocks)
	gen, kill := make([]bool, n), make([]bool, n)
	for bid, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Dst == l && in.Op.Yields() {
				kill[bid] = true
				break
			}
			if touchesHeld(in, l, held) {
				gen[bid] = true
				break
			}
		}
	}
	live := make([]bool, n)
	for changed := true; changed; {
		changed = false
		for bid := n - 1; bid >= 0; bid-- {
			if live[bid] || kill[bid] && !gen[bid] {
				continue
			}
			if gen[bid] || slices.ContainsFunc(f.Blocks[bid].Term.Targets, func(t midir.BlockID) bool { return live[t] }) {
				live[bid], changed = true, true
			}
		}
	}
	return live
}
