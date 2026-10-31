package mir

import "kigumi/internal/compiler/sem"

// A Never-typed value such as `panic()` ends its block although the jump after it stays in the IR.
type flowGraph struct {
	f     *Func
	never []bool
	preds [][]int
}

func newFlowGraph(f *Func) *flowGraph {
	g := &flowGraph{f: f, never: make([]bool, len(f.Blocks)), preds: make([][]int, len(f.Blocks))}
	for b, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == OpPanic || in.Op.Yields() && f.Locals[in.Dst].Type == sem.TyNever {
				g.never[b] = true
			}
		}
	}
	for b := range f.Blocks {
		for _, t := range g.succ(b) {
			g.preds[t] = append(g.preds[t], b)
		}
	}
	return g
}

func (g *flowGraph) succ(b int) []BlockID {
	if g.never[b] {
		return nil
	}
	return g.f.Blocks[b].Term.Targets
}
