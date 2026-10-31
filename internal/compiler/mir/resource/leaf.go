package resource

import "kigumi/internal/compiler/sem"

// trustedLeaf reports whether ent is a std bodiless primitive llgen inlines
// directly, not a real `extern` call.
func trustedLeaf(r *sem.Result, ent sem.EntityID) bool {
	e := r.Entity(ent)
	info := r.Fn(ent)
	if e.Flags&sem.EfStd == 0 || info.Abi != "" || len(info.Witnesses) > 0 {
		return false
	}
	for _, p := range info.Params {
		if k := r.Types.Kind(r.Entity(p).Type); k == sem.KFn || k == sem.KClosure {
			return false
		}
	}
	return true
}
