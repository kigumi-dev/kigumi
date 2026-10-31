package ownership

import "kigumi/internal/compiler/sem"

func IsAsyncBody(r *sem.Result, ent sem.EntityID) bool {
	e := r.Entity(ent)
	return e.Kind == sem.EntFn && r.Fn(ent).Declared&sem.EffAsync != 0
}

// A copy of one of these is a retain: the value is immutable, so no one can tell the two apart.
func RetainOnly(r *sem.Result, t sem.TypeID) bool {
	return r.Types.IsNumeric(t) || t == sem.TyBool || t == sem.TyChar || t == sem.TyString || t == sem.TyUnit
}
