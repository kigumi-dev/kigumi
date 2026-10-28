package mir

import "kigumi/internal/sem"

func (c *borrowCheck) callParams(g *Func, in Inst) ([]sem.TypeID, bool) {
	if in.Ent != 0 && c.p.R.Entity(in.Ent).Kind == sem.EntFn {
		var params []sem.TypeID
		for _, p := range c.declaredParams(in.Ent) {
			params = append(params, c.p.R.Entity(p).Type)
		}
		return params, true
	}
	if in.Op == OpCallValue && len(in.Args) > 0 {
		return c.calleeParams(g.Locals[in.Args[0]].Type)
	}
	return nil, false
}

func (c *borrowCheck) callParam(g *Func, in Inst, k int) (sem.TypeID, bool) {
	if in.Op == OpCallValue {
		k--
	}
	if k < 0 || in.Op != OpCall && in.Op != OpCallValue {
		return 0, false
	}
	params, known := c.callParams(g, in)
	if !known || k >= len(params) {
		return 0, false
	}
	return params[k], params[k] != 0
}
