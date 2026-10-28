package mir

import "kigumi/internal/sem"

func (c *borrowCheck) keptSignature(k *keptCtx, in Inst, e sem.EntityID, args []LocalID, off int) (uint64, func(int) uint64) {
	g := k.g
	var from uint64
	for _, a := range in.Args {
		from |= k.facts[a]
	}
	var res uint64
	ref := in.Op.Yields() && (c.holdsBorrow(g.Locals[in.Dst].Type) || c.ifaceRequirement(e) && c.holdsDeferred(g.Locals[in.Dst].Type))
	for _, a := range args {
		if ref || c.p.R.Types.Kind(g.Locals[a].Type) != sem.KRef && !g.Locals[a].Cell {
			res |= k.facts[a]
		} else {
			res |= c.moved(k, a)
		}
	}
	return res, func(i int) uint64 {
		if i >= off && (c.lendsPlace(g, in.Args[i]) || c.ifaceRequirement(e) && c.mayHoldPlace(g, in.Args[i])) {
			return from
		}
		return 0
	}
}
