package vm

import (
	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

// initGlobals materializes every owned/exported static as an immortal (rc -1) object; an extern static gets no VM storage.
func (m *Machine) initGlobals() {
	for _, ent := range m.r.StaticEntities() {
		if owner, _ := m.r.StaticOwnerOf(ent); owner == sem.StaticExtern {
			continue
		}
		lit, _ := m.r.StaticValueOf(ent)
		v := m.constant(&mir.Inst{Lit: lit, Type: m.r.Entity(ent).Type})
		m.immortalize(v)
		m.sh.globals[ent] = v
	}
}

// The key set is fixed once the run starts, so only the slot a `static mut`
// write replaces needs the lock.
func (m *Machine) loadGlobal(ent sem.EntityID) *obj {
	m.sh.store.Lock()
	v, ok := m.sh.globals[ent]
	m.sh.store.Unlock()
	if ok {
		return v
	}
	panic(&Unsupported{What: "reading the extern static `" + m.r.Entity(ent).Name + "` in the VM"})
}

func (m *Machine) storeGlobal(ent sem.EntityID, v *obj) {
	m.immortalize(v)
	m.sh.store.Lock()
	old, ok := m.sh.globals[ent]
	if ok {
		m.sh.globals[ent] = v
	}
	m.sh.store.Unlock()
	if !ok {
		panic(&Unsupported{What: "writing the extern static `" + m.r.Entity(ent).Name + "` in the VM"})
	}
	m.release(old)
}
