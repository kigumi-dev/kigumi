package vm

// tryFamily runs the fallible collection and text primitives. One fault tick
// per call stands for the allocation the native runtime may refuse; nil for other keys.
func (m *Machine) tryFamily(key string, a []*obj, a0, a1, a2 *obj) *obj {
	switch key {
	case "array.Array.tryWithCapacity":
		if uint64(a0.i) >= maxVMAlloc || m.birthRefused(key, a0.i) {
			return m.allocError()
		}
		return m.okTry(key, 1, newArray(key, a0.i, nil))
	case "array.Array.trySlice":
		if a1.i < 0 || a1.i > a2.i || a2.i > int64(len(a0.fields)) {
			return m.okTry(key, 1, m.none())
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
		return m.okCopied(key, a0.opaqueData().(*sharedCtrl).cell.inner)
	case "alloc.Shared.tryDowngrade":
		if m.planRefused(key) {
			return m.allocError()
		}
		c := a0.opaqueData().(*sharedCtrl)
		c.weak++
		return m.okPlanned(key, mkOpaque("Weak", c))
	case "prelude.AllocError.wrap":
		return m.okTry(key, 1, retain(a[0]))
	case "prelude.String.trySliceBytes":
		s, e := a1.i, a2.i
		if s < 0 || s > e || e > int64(len(a0.str())) || !boundary(a0.str(), s) || !boundary(a0.str(), e) {
			return m.okTry(key, 1, m.none())
		}
		if m.planRefused(key) {
			return m.allocError()
		}
		return m.okPlanned(key, m.some(mkStr(append([]byte{}, a0.str()[s:e]...))))
	case "prelude.Bytes.trySlice":
		s, e := a1.i, a2.i
		if s < 0 || s > e || e > int64(len(a0.str())) {
			return m.okTry(key, 1, m.none())
		}
		if m.planRefused(key) {
			return m.allocError()
		}
		return m.okPlanned(key, m.some(mkBytes(append([]byte{}, a0.str()[s:e]...))))
	case "map.Map.tryClone", "os.OsString.tryClone":
		return m.okCopied(key, a0)
	case "map.Map.tryEntries":
		return m.tryMapEntries(key, a0)
	case "map.tryEmpty":
		return m.tryEmptyRecord(key, "std/map", "Map")
	case "text.StringBuilder.tryNew":
		return m.tryEmptyRecord(key, "std/text", "StringBuilder")
	}
	return nil
}

// wrapRefused takes the two allocations (header, payload slots) of each variant layer native builds around a result.
func (m *Machine) wrapRefused(layers int) bool { return m.ticks(2 * layers) }

// bareWrapRefused is wrapRefused for the ops with a direct result form, which skip the variant when the call site takes the payload bare.
func (m *Machine) bareWrapRefused(layers int) bool {
	bare := m.tryBare
	m.tryBare = false
	return !bare && m.wrapRefused(layers)
}

func (m *Machine) okCopied(key string, v *obj) *obj {
	c, ok := m.copyTry(v)
	if !ok {
		return m.allocError()
	}
	return m.okTry(key, 1, c)
}

func (m *Machine) tryOptionalCopy(key string, src *obj, start, end int64) *obj {
	if m.birthRefused(key, end-start) {
		return m.allocError()
	}
	items := make([]*obj, 0, end-start)
	for i := start; i < end; i++ {
		x, ok := m.copyTry(src.fields[i])
		if !ok {
			m.releaseAll(items)
			return m.allocError()
		}
		items = append(items, x)
	}
	return m.okTry(key, 2, m.some(m.sized(newArray(key, end-start, items))))
}

func (m *Machine) releaseAll(items []*obj) {
	for _, x := range items {
		m.release(x)
	}
}

func copyItems(m *Machine, src *obj, start, end int64) *obj {
	out := make([]*obj, 0, end-start)
	for i := start; i < end; i++ {
		out = append(out, m.copy(src.fields[i]))
	}
	return m.sized(mkArray(out))
}

func (m *Machine) tryMapEntries(key string, mp *obj) *obj {
	keys, values := deref(mp.fields[0]), deref(mp.fields[1])
	if m.birthRefused(key, int64(len(keys.fields))) {
		return m.allocError()
	}
	entry := m.lookup("std/map", "Entry")
	out := make([]*obj, len(keys.fields))
	for i := range out {
		if m.ticks(2) {
			m.releaseAll(out[:i])
			return m.allocError()
		}
		k, ok := m.copyTry(keys.fields[i])
		var v *obj
		if ok {
			v, ok = m.copyTry(values.fields[i])
		}
		if !ok {
			m.releaseAll(out[:i])
			m.release(k)
			return m.allocError()
		}
		out[i] = mkRecord(entry, []*obj{k, v})
	}
	return m.okTry(key, 1, m.sized(newArray(key, int64(len(out)), out)))
}

// tryEmptyRecord builds a std record whose fields are all empty arrays: the record's header and slots, then each array's header.
func (m *Machine) tryEmptyRecord(key, pkg, name string) *obj {
	ent := m.lookup(pkg, name)
	fields := make([]*obj, len(m.r.TypeDecl(ent).Fields))
	if m.ticks(2 + len(fields)) {
		return m.allocError()
	}
	for i := range fields {
		fields[i] = mkArray(nil)
	}
	return m.okTry(key, 1, mkRecord(ent, fields))
}

func (m *Machine) sized(v *obj) *obj {
	m.chargeMemory(v)
	return v
}
