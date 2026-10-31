package mir

import (
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

// ResolveStdOp resolves a std entity to its typed id (at most one result
// is non-zero); ok is false for an entity the registry does not cover.
func ResolveStdOp(r *sem.Result, fn sem.EntityID) (opdef.PrimitiveId, opdef.ProviderOpId, bool) {
	id, ok := opdef.StdOpFor(StdKey(r, fn, true))
	if !ok {
		return 0, 0, false
	}
	if id.Kind() == opdef.KindProvider {
		return 0, opdef.ProviderOpId(id) + 1, true
	}
	return opdef.PrimitiveId(id) + 1, 0, true
}

// StdOp reports the native op id a resolved Inst carries, if any.
func (in *Inst) StdOp() (opdef.StdOpId, bool) {
	if in.Prim != 0 {
		return opdef.StdOpId(in.Prim - 1), true
	}
	if in.Prov != 0 {
		return opdef.StdOpId(in.Prov - 1), true
	}
	return 0, false
}
