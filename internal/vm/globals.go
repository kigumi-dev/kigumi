package vm

import (
	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

// initGlobals materializes every owned/exported static as an immortal (rc -1) object; an extern static gets no VM storage.
func (m *Machine) initGlobals() {
	m.globals = map[sem.EntityID]*obj{}
	for _, ent := range m.r.StaticEntities() {
		if owner, _ := m.r.StaticOwnerOf(ent); owner == sem.StaticExtern {
			continue
		}
		lit, _ := m.r.StaticValueOf(ent)
		v := m.constant(&mir.Inst{Lit: lit, Type: m.r.Entity(ent).Type})
		v.rc = -1
		m.globals[ent] = v
	}
}

func (m *Machine) loadGlobal(ent sem.EntityID) *obj {
	if v, ok := m.globals[ent]; ok {
		return v
	}
	panic(&Unsupported{What: "reading the extern static `" + m.r.Entity(ent).Name + "` in the VM"})
}

func (m *Machine) storeGlobal(ent sem.EntityID, v *obj) {
	old, ok := m.globals[ent]
	if !ok {
		panic(&Unsupported{What: "writing the extern static `" + m.r.Entity(ent).Name + "` in the VM"})
	}
	m.release(old)
	v.rc = -1
	m.globals[ent] = v
}
