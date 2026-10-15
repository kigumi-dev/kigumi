package vm

// collection implements the array primitives; nil for other keys.
func (m *Machine) collection(key string, a []*obj, a0, a1, a2 *obj) *obj {
	switch key {
	case "array.Array.empty":
		return mkArray(nil)
	case "array.Array.of", "prelude.FixedArray.adopt":
		return retain(a0)
	// No fault tick here: growBy is the fallible reservation push commits
	// into, same as native's try_grow fast path once capacity is enough.
	case "array.Array.push":
		a0.fields = append(a0.fields, retain(a[1]))
		// Growth happens in place on a0, not on the Unit this returns, so
		// the sandbox's per-call chargeMemory(result) never sees it.
		m.chargeMemory(a0)
		return unitObj
	case "array.Array.growBy":
		return mkBool(uint64(a1.i) < maxVMAlloc && !m.faultTick())
	case "array.Array.len":
		return mkInt(int64(len(a0.fields)), 64)
	case "array.Array.get":
		if a1.i < 0 || a1.i >= int64(len(a0.fields)) {
			return m.none()
		}
		return m.some(m.copy(a0.fields[a1.i]))
	case "array.Array.first":
		if len(a0.fields) == 0 {
			return m.none()
		}
		return m.some(m.copy(a0.fields[0]))
	case "array.Array.last":
		if len(a0.fields) == 0 {
			return m.none()
		}
		return m.some(m.copy(a0.fields[len(a0.fields)-1]))
	case "array.Array.with":
		if a1.i < 0 || a1.i >= int64(len(a0.fields)) {
			return m.none()
		}
		return m.some(m.callValue(a2, []*obj{a0.fields[a1.i]}))
	case "array.Array.withMut", "prelude.FixedArray.withMut":
		if a1.i < 0 || a1.i >= int64(len(a0.fields)) {
			return m.none()
		}
		// A scalar element aliases its obj on copy, so the callback
		// writes through a cell instead; the slot takes it back after.
		cell := mkCell(a0.fields[a1.i])
		defer func() {
			a0.fields[a1.i], cell.inner = cell.inner, nil
			m.release(cell)
		}()
		return m.some(m.callValue(a2, []*obj{cell}))
	case "array.Array.takeAt":
		i := a1.i
		if i < 0 || i >= int64(len(a0.fields)) {
			return m.none()
		}
		x := a0.fields[i]
		a0.fields = append(a0.fields[:i], a0.fields[i+1:]...)
		m.chargeMemory(a0)
		return m.some(x)
	case "array.Array.clone":
		if m.faultTick() {
			m.abort("out of memory")
		}
		return m.copy(a0)
	}
	return nil
}
