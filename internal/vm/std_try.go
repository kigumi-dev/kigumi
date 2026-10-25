package vm

// tryFamily runs the fallible collection and text primitives. One fault tick
// per call stands for the allocation the native runtime may refuse; nil for other keys.
func (m *Machine) tryFamily(key string, a []*obj, a0, a1, a2 *obj) *obj {
	switch key {
	case "array.Array.tryWithCapacity":
		if uint64(a0.i) >= maxVMAlloc || m.birthRefused(key, a0.i) {
			return m.allocError()
		}
		return m.okBare(newArray(key, a0.i, nil))
	case "array.Array.trySlice":
		if a1.i < 0 || a1.i > a2.i || a2.i > int64(len(a0.fields)) {
			return m.ok(m.none())
		}
		return m.tryOptionalCopy(key, a0, a1.i, a2.i)
	case "array.Array.tryMap":
		return m.tryMapItems(key, a0, a1)
	case "array.Array.tryFilter":
		return m.tryFilterItems(key, a0, a1)
	case "array.Array.tryInsertAt":
		at, n := uint64(a1.i), uint64(len(a0.fields))
		if m.wrapRefused(1) {
			return m.allocError()
		}
		if at > n {
			return m.ok(mkBool(false))
		}
		if !m.growFor(key, a0, nil) {
			return m.allocError()
		}
		a0.fields = append(a0.fields, nil)
		copy(a0.fields[at+1:], a0.fields[at:])
		a0.fields[at] = retain(a[2])
		m.chargeMemory(a0)
		return m.ok(mkBool(true))
	case "alloc.Shared.tryGet":
		if m.faultTick() {
			return m.allocError()
		}
		return m.ok(m.copy(a0.opaqueData().(*sharedCtrl).cell.inner))
	case "alloc.Shared.tryDowngrade":
		if m.faultTick() {
			return m.allocError()
		}
		c := a0.opaqueData().(*sharedCtrl)
		c.weak++
		return m.ok(mkOpaque("Weak", c))
	case "prelude.AllocError.wrap":
		return m.okBare(retain(a[0]))
	case "prelude.String.trySliceBytes":
		s, e := a1.i, a2.i
		if s < 0 || s > e || e > int64(len(a0.str())) || !boundary(a0.str(), s) || !boundary(a0.str(), e) {
			return m.ok(m.none())
		}
		if m.faultTick() {
			return m.allocError()
		}
		return m.ok(m.some(mkStr(append([]byte{}, a0.str()[s:e]...))))
	case "prelude.Bytes.trySlice":
		s, e := a1.i, a2.i
		if s < 0 || s > e || e > int64(len(a0.str())) {
			return m.ok(m.none())
		}
		if m.faultTick() {
			return m.allocError()
		}
		return m.ok(m.some(mkBytes(append([]byte{}, a0.str()[s:e]...))))
	case "map.Map.tryClone", "os.OsString.tryClone":
		if m.faultTick() {
			return m.allocError()
		}
		return m.ok(m.copy(a0))
	case "map.Map.tryEntries":
		return m.tryMapEntries(key, a0)
	case "map.tryEmpty":
		return m.tryEmptyRecord("std/map", "Map", 5)
	case "text.StringBuilder.tryNew":
		return m.tryEmptyRecord("std/text", "StringBuilder", 1)
	}
	return nil
}

// wrapRefused takes the two allocations (header, payload slots) of each variant layer native builds around a result.
func (m *Machine) wrapRefused(layers int) bool {
	for range 2 * layers {
		if m.faultTick() {
			return true
		}
	}
	return false
}

// bareWrapRefused is wrapRefused for the ops with a direct result form (tryWithCapacity, trySlice,
// AllocError.wrap), which skip the variant when the call site takes the payload bare.
func (m *Machine) bareWrapRefused(layers int) bool {
	bare := m.tryBare
	m.tryBare = false
	return !bare && m.wrapRefused(layers)
}

func (m *Machine) okWrapped(v *obj) *obj {
	if m.wrapRefused(1) {
		m.release(v)
		return m.allocError()
	}
	return m.ok(v)
}

func (m *Machine) okBare(v *obj) *obj {
	if m.bareWrapRefused(1) {
		m.release(v)
		return m.allocError()
	}
	return m.ok(v)
}

func (m *Machine) tryOptionalCopy(key string, src *obj, start, end int64) *obj {
	if m.birthRefused(key, end-start) {
		return m.allocError()
	}
	out := m.sized(newArray(key, end-start, copyList(m, src, start, end)))
	if m.bareWrapRefused(2) {
		m.release(out)
		return m.allocError()
	}
	return m.ok(m.some(out))
}

func copyList(m *Machine, src *obj, start, end int64) []*obj {
	out := make([]*obj, 0, end-start)
	for i := start; i < end; i++ {
		out = append(out, m.copy(src.fields[i]))
	}
	return out
}

func copyItems(m *Machine, src *obj, start, end int64) *obj {
	return m.sized(mkArray(copyList(m, src, start, end)))
}

func (m *Machine) tryMapEntries(key string, mp *obj) *obj {
	if m.birthRefused(key, int64(len(deref(mp.fields[0]).fields))) {
		return m.allocError()
	}
	keys, values := deref(mp.fields[0]), deref(mp.fields[1])
	entry := m.lookup("std/map", "Entry")
	out := make([]*obj, len(keys.fields))
	for i := range out {
		if m.faultTick() || m.faultTick() {
			for _, built := range out[:i] {
				m.release(built)
			}
			return m.allocError()
		}
		out[i] = mkRecord(entry, []*obj{m.copy(keys.fields[i]), m.copy(values.fields[i])})
	}
	return m.okWrapped(m.sized(newArray(key, int64(len(out)), out)))
}

// tryEmptyRecord builds a std record whose n fields are all empty arrays.
func (m *Machine) tryEmptyRecord(pkg, name string, n int) *obj {
	if m.faultTick() {
		return m.allocError()
	}
	fields := make([]*obj, n)
	for i := range fields {
		fields[i] = mkArray(nil)
	}
	return m.ok(mkRecord(m.lookup(pkg, name), fields))
}

func (m *Machine) sized(v *obj) *obj {
	m.chargeMemory(v)
	return v
}
