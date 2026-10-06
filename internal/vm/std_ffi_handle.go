package vm

import "kigumi/internal/sem"

// handleSlot is one HandleTable slot; generation increments each time the
// slot is freed, so a RawHandle naming the old generation is rejected.
type handleSlot struct {
	generation int64
	occupied   bool
	value      *obj
}

type handleTableState struct {
	slots []handleSlot
	free  []int
}

func packSlot(generation, index int64) int64       { return generation<<32 | index }
func unpackSlot(v int64) (generation, index int64) { return v >> 32, v & 0xFFFFFFFF }

// Like Pin, a slot holds a boxed VM value, but also a generation a
// stale or forged RawHandle must match.
func (m *Machine) registerFFIHandle() {
	h := m.host
	tableIdx := func(v *obj) int64 { return deref(deref(v).fields[0]).i }
	table := func(m *Machine, v *obj) *handleTableState { return m.handleTable(tableIdx(v)) }
	// Only an unsafe reclaim of a forged/stale RawHandle can fail this
	// check; a live FfiHandle otherwise always names a live slot.
	handleLoc := func(m *Machine, v *obj) (*handleTableState, int64) {
		r := deref(v)
		t := m.handleTable(deref(r.fields[0]).i)
		gen, slot := unpackSlot(deref(r.fields[1]).i)
		if slot < 0 || int(slot) >= len(t.slots) || !t.slots[slot].occupied || t.slots[slot].generation != gen {
			m.abort("this handle does not address a live slot of this table")
		}
		return t, slot
	}
	// remove/drop zero this (0 is never a real 1-based table) so the
	// scope-exit FfiHandle.drop a primitive call still lends h to is a no-op (see Pin.release).
	clearHandle := func(v *obj) { deref(v).fields[0].i = 0 }
	free := func(t *handleTableState, slot int64) *obj {
		v := t.slots[slot].value
		t.slots[slot] = handleSlot{generation: t.slots[slot].generation + 1}
		t.free = append(t.free, int(slot))
		return v
	}

	h["ffi.HandleTable.new"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		return mkRecord(m.lookup("std/ffi", "HandleTable"), []*obj{mkInt(m.addHandleTable(), m.numKind(sem.TyI64))})
	}
	h["ffi.HandleTable.insert"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		ti, t, v := tableIdx(a[0]), table(m, a[0]), retain(deref(a[1]))
		var slot int64
		if n := len(t.free); n > 0 {
			slot = int64(t.free[n-1])
			t.free = t.free[:n-1]
			t.slots[slot].occupied, t.slots[slot].value = true, v
		} else {
			slot = int64(len(t.slots))
			t.slots = append(t.slots, handleSlot{occupied: true, value: v})
		}
		raw := packSlot(t.slots[slot].generation, slot)
		return mkRecord(m.lookup("std/ffi", "FfiHandle"), []*obj{mkInt(ti, m.numKind(sem.TyI64)), mkInt(raw, m.numKind(sem.TyI64))})
	}
	h["ffi.HandleTable.with"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		t, slot := handleLoc(m, a[1])
		return m.callValue(a[2], []*obj{t.slots[slot].value})
	}
	h["ffi.HandleTable.remove"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		t, slot := handleLoc(m, a[1])
		v := free(t, slot)
		clearHandle(a[1])
		return v
	}
	h["ffi.HandleTable.reclaim"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		ti, raw := tableIdx(a[0]), deref(a[1])
		rawTable, rawSlot := deref(raw.fields[0]).i, deref(raw.fields[1]).i
		if rawTable != ti {
			return m.handleFault("WrongTable")
		}
		gen, slot := unpackSlot(rawSlot)
		t := m.handleTable(ti)
		if slot < 0 || int(slot) >= len(t.slots) {
			return m.handleFault("OutOfRange")
		}
		if !t.slots[slot].occupied || t.slots[slot].generation != gen {
			return m.handleFault("Stale")
		}
		return m.ok(mkRecord(m.lookup("std/ffi", "FfiHandle"), []*obj{mkInt(ti, m.numKind(sem.TyI64)), mkInt(rawSlot, m.numKind(sem.TyI64))}))
	}
	h["ffi.HandleTable.close"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		t := table(m, a[0])
		if live := len(t.slots) - len(t.free); live > 0 {
			busy := mkRecord(m.lookup("std/ffi", "HandleTableBusy"), []*obj{mkInt(int64(live), m.numKind(sem.TyUsize))})
			return mkVariant(m.variantOf(m.r.Types.ResultEnt(), "Err"), []*obj{busy})
		}
		return m.ok(unitObj)
	}
	h["ffi.HandleTable.drop"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		t := table(m, a[0])
		if live := len(t.slots) - len(t.free); live > 0 {
			m.abort("dropped a HandleTable with a handle still outstanding; call close() first")
		}
		return unitObj
	}
	h["ffi.FfiHandle.raw"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		r := deref(a[0])
		return mkRecord(m.lookup("std/ffi", "RawHandle"), []*obj{mkInt(deref(r.fields[0]).i, m.numKind(sem.TyI64)), mkInt(deref(r.fields[1]).i, m.numKind(sem.TyI64))})
	}
	h["ffi.FfiHandle.drop"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		if deref(a[0]).fields[0].i == 0 {
			return unitObj
		}
		t, slot := handleLoc(m, a[0])
		m.release(free(t, slot))
		return unitObj
	}
}

// handleFault builds Err(HandleFault.<variant>).
func (m *Machine) handleFault(variant string) *obj {
	v := mkVariant(m.variantOf(m.lookup("std/ffi", "HandleFault"), variant), nil)
	return mkVariant(m.variantOf(m.r.Types.ResultEnt(), "Err"), []*obj{v})
}
