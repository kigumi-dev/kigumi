package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// storesNew reports whether the call may start a Task and leave it in the place argument k lends: a callee whose
// summary says so, or one not known here, when that place's type can hold a Task.
func (r *taskRun) storesNew(cs taskCallees, in *midir.Inst, k int) bool {
	may := cs.unknown
	for i := range cs.known {
		ce := &cs.known[i]
		if j := k - ce.off; j >= 0 && (ce.fate.all || ce.fate.stores.has(j)) {
			may = true
		}
	}
	if !may {
		return false
	}
	t := r.g.Locals[in.Args[k]].Type
	if elem, ok := r.c.p.R.MutRefElem(t); ok {
		t = elem
	}
	return r.c.p.R.TypeHoldsTask(t) || r.c.p.R.OpenType(t)
}
