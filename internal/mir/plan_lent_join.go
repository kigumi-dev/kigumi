package mir

import "kigumi/internal/sem"

// lentJoins finds the reference temporaries that join branches where at least
// one branch is lent: the join holds no count, and a counted branch keeps its own.
func (p *Program) lentJoins(f *Func, defs map[LocalID]int) map[LocalID]bool {
	var lent []bool
	out := map[LocalID]bool{}
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
