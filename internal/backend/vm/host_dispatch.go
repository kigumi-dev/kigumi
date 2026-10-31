package vm

import (
	"strings"

	"kigumi/internal/compiler/mir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

// indexHostByOp skips build.* keys: StdOpFor maps every one to the same BUILD_ONLY id.
func (m *Machine) indexHostByOp() {
	m.hostByOp = map[opdef.ProviderOpId]hostFn{}
	for key, fn := range m.host {
		if strings.HasPrefix(key, "build.") {
			continue
		}
		if id, ok := opdef.StdOpFor(key); ok && id.Kind() == opdef.KindProvider {
			m.hostByOp[opdef.ProviderOpId(id)+1] = fn
		}
	}
}

// hostFor falls back to key for host entries the registry does not cover.
func (m *Machine) hostFor(prov opdef.ProviderOpId, key string) (hostFn, bool) {
	if prov != 0 {
		if h, ok := m.hostByOp[prov]; ok {
			return h, true
		}
	}
	h, ok := m.host[key]
	return h, ok
}

// hostForEnt is hostFor for a callee only known at runtime (interface dispatch, resource drop).
func (m *Machine) hostForEnt(ent sem.EntityID, key string) (hostFn, bool) {
	_, prov, _ := mir.ResolveStdOp(m.r, ent)
	return m.hostFor(prov, key)
}
