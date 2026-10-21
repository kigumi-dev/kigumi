package sem

import "kigumi/internal/syntax"

func (c *checker) asmEdges(n syntax.NodeID, info *AsmInfo) {
	e := EffectEdge{Kind: EdgeAsm, Node: n}
	if c.naked {
		e.Why = asmNaked
	}
	c.edge(e)
	for _, op := range info.Operands {
		if op.Kind == AsmSym && op.Sym != 0 {
			c.edge(EffectEdge{Kind: EdgeAsm, Param: op.Sym, Node: op.Node})
		}
	}
}
