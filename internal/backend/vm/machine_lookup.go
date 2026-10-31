package vm

import (
	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
)

// lookup finds a std declaration by package path and name; lang items are
// resolved lazily by the checker, so the VM finds them itself.
func (m *Machine) lookup(pkg, name string) sem.EntityID {
	key := pkg + "." + name
	m.sh.derived.Lock()
	ent, ok := m.sh.named[key]
	m.sh.derived.Unlock()
	if ok {
		return ent
	}
	var found sem.EntityID
	for id := 1; id < len(m.r.Entities); id++ {
		e := &m.r.Entities[id]
		if e.Name == name && e.Kind == sem.EntType && m.r.Packages[e.Pkg].Path == pkg {
			found = sem.EntityID(id)
			break
		}
	}
	m.sh.derived.Lock()
	m.sh.named[key] = found
	m.sh.derived.Unlock()
	return found
}

func (m *Machine) variantOf(adt sem.EntityID, name string) sem.EntityID {
	for _, v := range m.r.TypeDecl(adt).Variants {
		if m.r.Entity(v).Name == name {
			return v
		}
	}
	return 0
}

func (m *Machine) isResource(ent sem.EntityID) bool {
	return m.r.TypeDecl(ent).Form == sem.FormResource
}

// hostDrop is the accelerator standing in for a bodiless std destructor
// (ffi.Pin.drop, ffi.CValue.drop).
func (m *Machine) hostDrop(ent sem.EntityID) (hostFn, sem.EntityID) {
	if !m.isResource(ent) {
		return nil, 0
	}
	drop := m.r.TypeDecl(ent).Drop
	if drop == 0 {
		return nil, 0
	}
	h, _ := m.hostForEnt(drop, m.stdKey(drop))
	return h, drop
}

// dropFn is the destructor a resource type declares, lowered; nil for a
// type without one or with a runtime-only one.
func (m *Machine) dropFn(ent sem.EntityID) *mir.Func {
	m.sh.derived.Lock()
	f, ok := m.sh.drops[ent]
	m.sh.derived.Unlock()
	if ok {
		return f
	}
	if m.isResource(ent) {
		if drop := m.r.TypeDecl(ent).Drop; drop != 0 {
			f, _ = m.lowered(drop)
		}
	}
	m.sh.derived.Lock()
	m.sh.drops[ent] = f
	m.sh.derived.Unlock()
	return f
}
