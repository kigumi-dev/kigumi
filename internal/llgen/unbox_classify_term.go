package llgen

import "kigumi/internal/mir"

// classifyTerm lets a Bool TermBranch condition read straight from its i1
// register; TermReturn always boxes on demand, crossing the C ABI.
func classifyTerm(t mir.Term, out map[mir.LocalID]scalarInfo, disqualify func(mir.LocalID)) {
	switch t.Op {
	case mir.TermBranch:
		if info, ok := out[t.Args[0]]; ok && info.Ty != TI1 {
			disqualify(t.Args[0])
		}
	}
}

func (e *emitter) inPlaceBuiltin(in mir.Inst) bool {
	switch in.Str {
	case "fixed.fill", "direct.eq", "direct.hash":
		return true
	case "atomic.load", "atomic.store", "atomic.exchange", "atomic.compareExchange", "atomic.add", "atomic.sub", "atomic.and", "atomic.or", "atomic.xor", "volatile.read", "volatile.write":
		return true
	case "iter.len", "iter.at", "iter.at_ref", "iter.at_move":
		return e.isDirect(in.Args[0])
	}
	return false
}
