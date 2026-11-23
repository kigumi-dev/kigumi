package sem

import "kigumi/internal/compiler/syntax"

func (c *checker) asmEdges(n syntax.NodeID, info *AsmInfo) {
	e := EffectEdge{Kind: EdgeAsm, Node: n}
	if c.naked {
		e.Why = asmNaked
	}
	c.edge(e)
	// A record result the backend does not keep unboxed is built with a box
	// per output; a direct aggregate is written in place instead.
	if info.Result != 0 && !c.r.DirectAgg(info.Result) && c.r.BoxAllocates(info.Result) {
		c.edge(EffectEdge{Kind: EdgeAlloc, Node: n, Type: info.Result})
	}
	for _, op := range info.Operands {
		if op.Kind == AsmSym && op.Sym != 0 {
			c.edge(EffectEdge{Kind: EdgeAsm, Param: op.Sym, Node: op.Node})
		}
	}
}
