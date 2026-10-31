package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// lentJoins finds the reference temporaries that join branches where at least
// one branch is lent: the join holds no count, and a counted branch keeps its own.
func (p *Program) lentJoins(f *midir.Func, defs map[midir.LocalID]int) map[midir.LocalID]bool {
	var lent []bool
	out := map[midir.LocalID]bool{}
	for t, n := range defs {
		if n < 2 || p.R.Types.Kind(f.Locals[t].Type) != sem.KRef {
			continue
		}
		if lent == nil {
			lent = p.lentRefs(f)
		}
		if lent[t] {
			out[t] = true
		}
	}
	return out
}
