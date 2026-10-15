package llgen

import "kigumi/internal/sem"

// usesInt128 reports whether any emitted function holds a 128-bit integer, the one case the runtime needs its 128-bit support.
func (e *emitter) usesInt128() bool {
	seen := map[sem.TypeID]bool{}
	for _, f := range e.p.Funcs {
		if e.reachable != nil && !e.reachable.Keep(f.Ent) {
			continue
		}
		for _, l := range f.Locals {
			if e.holdsInt128(l.Type, seen) || e.holdsInt128(e.r.UnderlyingRepr(l.Type), seen) {
				return true
			}
		}
	}
	return false
}

// holdsInt128 looks inside records, whose 128-bit fields are boxed and unboxed by the runtime wherever the record crosses.
func (e *emitter) holdsInt128(t sem.TypeID, seen map[sem.TypeID]bool) bool {
	if t == 0 || seen[t] {
		return false
	}
	seen[t] = true
	tt := e.r.Types
	if tt.Contains(t, sem.TyI128) || tt.Contains(t, sem.TyU128) {
		return true
	}
	if elem, _, ok := e.r.FixedElem(t); ok {
		return e.holdsInt128(elem, seen)
	}
	if e.isCRecord(t) {
		for _, f := range e.r.TypeDecl(tt.Node(t).Ent).Fields {
			if e.holdsInt128(e.r.Entity(f).Type, seen) {
				return true
			}
		}
		return false
	}
	if !e.r.DirectAgg(t) {
		return false
	}
	if e.r.DirectAdt(t) {
		for _, v := range e.r.AggVariants(t) {
			for _, p := range e.r.AggPayload(t, v) {
				if e.holdsInt128(p, seen) {
					return true
				}
			}
		}
		return false
	}
	for _, ft := range e.r.AggFields(t) {
		if e.holdsInt128(ft, seen) {
			return true
		}
	}
	return false
}
