package borrow

import (
	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/syntax"
)

type instPos struct {
	b midir.BlockID
	i int
}

// A store a drop makes is reported where the value was last written, not at its declaration.
func (c *borrowCheck) dropStoreNode(in midir.Inst) syntax.NodeID {
	if len(in.Args) == 0 {
		return in.Node
	}
	if c.preds == nil {
		c.preds = mirflow.Preds(c.f)
	}
	var found syntax.NodeID
	seen := map[midir.BlockID]bool{}
	for work := []instPos{{c.curBlock, c.curIdx}}; len(work) > 0; {
		p := work[len(work)-1]
		work = work[:len(work)-1]
		if n, ok := c.lastWrite(in.Args[0], p); ok {
			if n == 0 || found != 0 && n != found {
				return in.Node
			}
			found = n
			continue
		}
		if len(c.preds[p.b]) == 0 {
			return in.Node
		}
		for _, pb := range c.preds[p.b] {
			if !seen[pb] {
				seen[pb] = true
				work = append(work, instPos{pb, len(c.f.Blocks[pb].Insts)})
			}
		}
	}
	if found == 0 {
		return in.Node
	}
	return found
}

func (c *borrowCheck) lastWrite(l midir.LocalID, p instPos) (syntax.NodeID, bool) {
	insts := c.f.Blocks[p.b].Insts
	for i := p.i - 1; i >= 0; i-- {
		w := insts[i]
		if w.Op.Yields() && w.Dst == l || (w.Op == midir.OpSetField || w.Op == midir.OpSetIndex) && w.Args[0] == l {
			return w.Node, true
		}
	}
	return 0, false
}
