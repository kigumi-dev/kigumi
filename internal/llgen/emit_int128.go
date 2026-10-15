package llgen

import "kigumi/internal/sem"

// usesInt128 reports whether any emitted function holds a 128-bit integer, the one case the runtime needs its 128-bit support.
func (e *emitter) usesInt128() bool {
	tt := e.r.Types
	for _, f := range e.p.Funcs {
		if e.reachable != nil && !e.reachable.Keep(f.Ent) {
			continue
		}
		for _, l := range f.Locals {
			for _, t := range []sem.TypeID{l.Type, e.r.UnderlyingRepr(l.Type)} {
				if tt.Contains(t, sem.TyI128) || tt.Contains(t, sem.TyU128) {
					return true
				}
			}
		}
	}
	return false
}
