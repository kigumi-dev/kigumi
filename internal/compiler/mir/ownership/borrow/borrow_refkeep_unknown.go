package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (c *borrowCheck) unknownResultBorrows(in midir.Inst) []*borrowInfo {
	if in.Op != midir.OpCallValue || len(in.Args) == 0 {
		return nil
	}
	tt := c.p.R.Types
	sig := c.slotFnType(c.f.Locals[in.Args[0]].Type)
	if sig == 0 {
		return nil
	}
	var out []*borrowInfo
	for i, t := range tt.Node(sig).Args {
		if tt.Kind(t) != sem.KRef || i+1 >= len(in.Args) {
			continue
		}
		b, ok := c.tieInfo(flowArg{arg: in.Args[i+1], tie: tieRef, shared: !tt.RefMut(t)}, false, false)
		if ok {
			if b = c.carriedBorrows(b); b != nil {
				out = append(out, b)
			}
		}
	}
	return out
}
