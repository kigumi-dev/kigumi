package ops

import (
	"strings"
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
	stdOpID = func() map[string]StdOpId {
		m := make(map[string]StdOpId, len(StdOpNames))
		for i, name := range StdOpNames {
			m[name] = StdOpId(i)
		}
		return m
	}()
	stdOpKind = func() map[StdOpId]OpKind {
		m := map[StdOpId]OpKind{}
		for name := range providerOps {
			m[stdOpID[name]] = KindProvider
		}
		return m
	}()
	stdOpByKey = func() map[string]StdOpId {
		m := make(map[string]StdOpId, len(stdOpKeys))
		for key, name := range stdOpKeys {
			m[key] = stdOpID[name]
		}
		return m
	}()
	stdOpBySuffix = func() map[string]StdOpId {
		m := make(map[string]StdOpId, len(stdOpSuffixes))
		for method, name := range stdOpSuffixes {
			m[method] = stdOpID[name]
		}
		return m
	}()
)

type stdOpPrefix struct {
	prefix string
	id     StdOpId
}

var stdOpPrefixes = func() []stdOpPrefix {
	var out []stdOpPrefix
	for _, op := range Registry.Ops {
		for _, p := range op.Prefixes {
			out = append(out, stdOpPrefix{p, StdOpId(op.ID)})
		}
	}
	return out
}()

// Kind reports whether id is a compiler-known primitive or a host/provider
// operation.
func (id StdOpId) Kind() OpKind { return stdOpKind[id] }

// StdOpIdOf resolves a StdOpNames entry to its id, for a backend that has
// to name one operation in particular.
func StdOpIdOf(name string) (StdOpId, bool) {
	id, ok := stdOpID[name]
	return id, ok
}

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
	for _, p := range stdOpPrefixes {
		if strings.HasPrefix(key, p.prefix) {
			return p.id, true
		}
	}
	return 0, false
}
