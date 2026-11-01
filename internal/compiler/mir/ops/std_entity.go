package ops

import "kigumi/internal/compiler/sem"

// ResolveStdOp resolves a std entity to its typed id (at most one result
// is non-zero); ok is false for an entity the registry does not cover.
func ResolveStdOp(r *sem.Result, fn sem.EntityID) (PrimitiveId, ProviderOpId, bool) {
	id, ok := StdOpFor(StdKey(r, fn, true))
	if !ok {
		return 0, 0, false
	}
	if id.Kind() == KindProvider {
		return 0, ProviderOpId(id) + 1, true
	}
	return PrimitiveId(id) + 1, 0, true
}
