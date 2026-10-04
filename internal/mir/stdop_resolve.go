package mir

import (
	"strings"

	"kigumi/internal/sem"
)

// OpKind is KindPrimitive (same on every backend) or KindProvider (needs
// a Target/Capability).
type OpKind uint8

const (
	KindPrimitive OpKind = iota
	KindProvider
)

// StdOpId indexes StdOpNames and backs both PrimitiveId and ProviderOpId.
type StdOpId int32

// PrimitiveId is a resolved KindPrimitive callee: 0 means none, else 1+StdOpId.
type PrimitiveId int32

// ProviderOpId is a resolved KindProvider callee: 0 means none, else 1+StdOpId.
type ProviderOpId int32

var (
	stdOpID       = map[string]StdOpId{}
	stdOpKind     = map[StdOpId]OpKind{}
	stdOpByKey    = map[string]StdOpId{}
	stdOpBySuffix = map[string]StdOpId{}
)

func init() {
	for i, name := range StdOpNames {
		stdOpID[name] = StdOpId(i)
		if providerOps[name] {
			stdOpKind[StdOpId(i)] = KindProvider
		}
	}
	for key, name := range stdOpKeys {
		stdOpByKey[key] = stdOpID[name]
	}
	for method, name := range stdOpSuffixes {
		stdOpBySuffix[method] = stdOpID[name]
	}
}

// Kind reports whether id is a compiler-known primitive or a host/provider
// operation.
func (id StdOpId) Kind() OpKind { return stdOpKind[id] }

// StdOpFor resolves a std dispatch key to the id rt_std_id dispatches on.
func StdOpFor(key string) (StdOpId, bool) {
	key = strings.TrimPrefix(key, "std/")
	if id, ok := stdOpByKey[key]; ok {
		return id, true
	}
	if i := strings.LastIndexByte(key, '.'); i >= 0 {
		if id, ok := stdOpBySuffix[key[i+1:]]; ok {
			return id, true
		}
	}
	if strings.HasPrefix(key, "build.") {
		return stdOpID["BUILD_ONLY"], true
	}
	return 0, false
}

// ResolveStdOp resolves a std entity to its typed id (at most one result
// is non-zero); ok is false for an entity the registry does not cover.
func ResolveStdOp(r *sem.Result, fn sem.EntityID) (PrimitiveId, ProviderOpId, bool) {
	id, ok := StdOpFor(StdKey(r, fn, true))
	if !ok {
		return 0, 0, false
	}
	if stdOpKind[id] == KindProvider {
		return 0, ProviderOpId(id) + 1, true
	}
	return PrimitiveId(id) + 1, 0, true
}

// StdOp reports the native op id a resolved Inst carries, if any.
func (in *Inst) StdOp() (StdOpId, bool) {
	if in.Prim != 0 {
		return StdOpId(in.Prim - 1), true
	}
	if in.Prov != 0 {
		return StdOpId(in.Prov - 1), true
	}
	return 0, false
}
