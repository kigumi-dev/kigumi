package mir

import "slices"

type boundSlot struct {
	g *Func
	i int
}

func (c *borrowCheck) boundOpaqueStores(g *Func, i int, seen map[boundSlot]bool) bool {
	key := boundSlot{g, i}
	if seen[key] {
		return false
	}
	seen[key] = true
	params, fs := c.declaredParams(g.Ent), c.bodyFacts(g)
	if i >= len(params) || fs == nil {
		return true
	}
	source, defs := uint32(params[i]), soleDefs(g)
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			if in.Op != OpCall && in.Op != OpCallValue {
				continue
			}
			h, args := c.bodyCallee(in), in.Args
			if in.Op == OpCallValue {
				o := c.calleeOrigin(g, defs, in.Args[0])
				h, args = o.body, slices.Concat(o.captures, in.Args[1:])
				if h == nil && !o.known {
					for k, a := range in.Args {
						decl, _ := c.callParam(g, in, k)
						if fs[a][source] && (k == 0 || decl == 0 || c.paramKeepsBorrow(decl)) {
							return true
						}
					}
				}
			}
			if h == nil {
				continue
			}
			for k, a := range args {
				if fs[a][source] && c.boundOpaqueStores(h, k, seen) {
					return true
				}
			}
		}
	}
	return false
}
