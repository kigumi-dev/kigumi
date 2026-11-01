package mir

import "kigumi/internal/compiler/sem"

// LentMask says which arguments a std primitive used as a witness borrows
// rather than consumes (bit i for argument i): a `self` / `mut self`
// receiver and every `&T` parameter. rt_callv releases the others.
func LentMask(r *sem.Result, fn sem.EntityID) int {
	info := r.Fn(fn)
	mask, i := 0, 0
	if info.Recv != sem.RecvNone {
		if info.Recv != sem.RecvMove {
			mask |= 1
		}
		i = 1
	}
	for _, t := range r.Types.Node(info.Sig).Args {
		if r.Types.Kind(t) == sem.KRef {
			mask |= 1 << i
		}
		i++
	}
	return mask
}
