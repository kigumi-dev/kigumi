package mir

import "kigumi/internal/compiler/sem"

func (c *borrowCheck) flowCallee(in Inst) (sem.EntityID, []LocalID) {
	e, args := in.Ent, in.Args
	switch in.Op {
	case OpCall:
	case OpCallValue, OpFutureNew:
		if len(args) == 0 {
			return 0, nil
		}
		if e == 0 {
			if d := c.definition(args[0]); d != nil && d.Op == OpFnItem {
				e = d.Ent
			}
		}
		args = args[1:]
	default:
		return 0, nil
	}
	if e == 0 || c.p.R.Entity(e).Kind != sem.EntFn {
		return 0, nil
	}
	return e, args
}
