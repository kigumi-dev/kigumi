package vm

import (
	"sync/atomic"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// initGlobals materializes every owned/exported static as an immortal, rcShared when native shares it too; an extern static gets no VM storage.
func (m *Machine) initGlobals() {
	for _, ent := range m.r.StaticEntities() {
		if owner, _ := m.r.StaticOwnerOf(ent); owner == sem.StaticExtern {
			continue
		}
		lit, ok := m.r.StaticValueOf(ent)
		if !ok && m.r.StaticPending(ent) {
			continue
		}
		v := m.constant(&midir.Inst{Lit: lit, Type: m.r.Entity(ent).Type})
		if m.r.ImmortalStatic(ent) {
			m.pinDeep(v, rcShared)
		} else {
			m.immortalizeDeep(v)
		}
		m.sh.globals[ent] = v
	}
}

// A static's whole tree outlives the run, so none of it counts as live.
func (m *Machine) immortalizeDeep(v *obj) { m.pinDeep(v, rcImmortal) }

func (m *Machine) pinDeep(v *obj, rc int32) {
	if v == nil || atomic.LoadInt32(&v.rc) < 0 {
		return
	}
	m.pin(v, rc)
	for _, f := range v.fields {
		m.pinDeep(f, rc)
	}
	m.pinDeep(v.inner, rc)
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
	if m.r.StaticPending(ent) {
		panic(&Unsupported{What: "reading the static `" + m.r.Entity(ent).Name + "` before its `comptime` initializer has been evaluated"})
	}
	panic(&Unsupported{What: "reading the extern static `" + m.r.Entity(ent).Name + "` in the VM"})
}

func (m *Machine) storeGlobal(ent sem.EntityID, v *obj) {
	m.immortalizeDeep(v)
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

// resetStatics gives the next test the initial value of every static, so a `static mut` write never leaks between tests.
func (m *Machine) resetStatics() {
	m.sh.store.Lock()
	clear(m.sh.globals)
	m.sh.store.Unlock()
	m.initGlobals()
}
