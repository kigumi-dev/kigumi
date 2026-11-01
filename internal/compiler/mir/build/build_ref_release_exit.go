package build

import (
	"slices"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
)

type releasePoint struct{ block, at int }

// releaseAtExits puts one release of l on each way out of the blocks still using l or a borrow read from it: after
// the last use, at the start of a block entered only from them, or in a new block on an edge into a join.
func (g *ghostSet) releaseAtExits(f *midir.Func, l midir.LocalID, held map[midir.LocalID]bool, live []bool, home int, flow *blockFlow) bool {
	in := func(b int) bool { return live[b] || b == home }
	var points []releasePoint
	var edges []mir.Edge
	for b, blk := range f.Blocks {
		if !in(b) {
			continue
		}
		drop := slices.IndexFunc(blk.Insts, func(x midir.Inst) bool { return releasesRef(x, l) })
		if !flow.liveOut(b, live) {
			at := lastTouch(blk.Insts, l, held)
			switch {
			case drop >= 0 && drop < at:
				return false
			case len(blk.Term.Targets) > 0 || drop >= 0:
				points = append(points, releasePoint{b, at})
			case blk.Term.Op == midir.TermReturn:
				return false
			}
			continue
		}
		if drop >= 0 {
			return false
		}
		for slot, t := range blk.Term.Targets {
			switch {
			case int(t) == home:
				return false
			case in(int(t)):
			case len(flow.preds[t]) == 1:
				points = append(points, releasePoint{int(t), 0})
			default:
				edges = append(edges, mir.Edge{From: b, Slot: slot})
			}
		}
	}
	var tmpl midir.Inst
	for b := range f.Blocks {
		if i := slices.IndexFunc(f.Blocks[b].Insts, func(x midir.Inst) bool { return releasesRef(x, l) }); i >= 0 {
			tmpl = g.deleteInst(f, b, i)
		}
	}
	for _, pt := range points {
		at := 0
		if pt.at > 0 {
			at = lastTouch(f.Blocks[pt.block].Insts, l, held)
		}
		g.insertInst(f, pt.block, at, tmpl)
	}
	for _, e := range edges {
		to := f.Blocks[e.From].Term.Targets[e.Slot]
		f.Blocks = append(f.Blocks, midir.Block{Insts: []midir.Inst{tmpl}, Term: midir.Term{Op: midir.TermJump, Targets: []midir.BlockID{to}}})
		f.Blocks[e.From].Term.Targets[e.Slot] = midir.BlockID(len(f.Blocks) - 1)
	}
	return true
}
