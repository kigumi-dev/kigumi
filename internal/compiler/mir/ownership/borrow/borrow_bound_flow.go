package borrow

import (
	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
)

type boundSlot struct {
	g *midir.Func
	i int
}

func (c *borrowCheck) boundOpaqueStores(g *midir.Func, i int, seen map[boundSlot]bool) bool {
	key := boundSlot{g, i}
	if seen[key] {
		return false
	}
	seen[key] = true
	params, fs := c.declaredParams(g.Ent), c.bodyFacts(g)
	if i >= len(params) || fs == nil {
		return true
	}
	source, defs := uint32(params[i]), mirflow.SoleDefs(g)
	passes := func(h *midir.Func, args []midir.LocalID) bool {
		for k, a := range args {
			if h != nil && fs[a][source] && c.boundOpaqueStores(h, k, seen) {
				return true
			}
		}
		return false
	}
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			switch in.Op {
			case midir.OpCall:
				if passes(c.bodyCallee(in), in.Args) {
					return true
				}
			case midir.OpCallValue:
				origins, ok := c.calleeOrigins(g, defs, in.Args[0])
				if !ok || origins[0].isParam {
					for k, a := range in.Args {
						decl, _ := c.callParam(g, in, k)
						if fs[a][source] && (k == 0 || decl == 0 || c.paramKeepsBorrow(decl)) {
							return true
						}
					}
					continue
				}
				for _, o := range origins {
					if passes(o.body, o.params(in.Args[1:])) {
						return true
					}
				}
			}
		}
	}
	return false
}
