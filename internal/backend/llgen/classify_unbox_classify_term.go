package llgen

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// classifyTerm lets a Bool TermBranch condition read straight from its i1
// register; TermReturn always boxes on demand, crossing the C ABI.
func classifyTerm(t midir.Term, out map[midir.LocalID]scalarInfo, disqualify func(midir.LocalID)) {
	switch t.Op {
	case midir.TermBranch:
		if info, ok := out[t.Args[0]]; ok && info.Ty != TI1 {
			disqualify(t.Args[0])
		}
	}
}

func (e *emitter) inPlaceBuiltin(in midir.Inst) bool {
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
