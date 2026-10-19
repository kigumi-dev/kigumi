package vm

// tryFamily runs the fallible collection and text primitives. One fault tick
// per call stands for the allocation the native runtime may refuse; nil for other keys.
func (m *Machine) tryFamily(key string, a []*obj, a0, a1, a2 *obj) *obj {
	switch key {
	case "array.Array.tryWithCapacity":
		if uint64(a0.i) >= maxVMAlloc || m.faultTick() {
			return m.allocError()
		}
		return m.ok(mkArray(nil))
	case "array.Array.tryCopyRange":
		n := uint64(len(a0.fields))
		start, end := min(uint64(a1.i), n), min(uint64(a2.i), n)
		return m.tryCopyItems(a0, int64(start), int64(max(end, start)), false)
	case "array.Array.trySlice":
		if a1.i < 0 || a1.i > a2.i || a2.i > int64(len(a0.fields)) {
			return m.ok(m.none())
		}
		return m.tryOptionalCopy(a0, a1.i, a2.i)
	case "array.Array.tryMap":
		return m.tryMapItems(a0, a1)
	case "array.Array.tryFilter":
		return m.tryFilterItems(a0, a1)
	case "array.Array.tryConcat":
		if m.faultTick() {
			return m.allocError()
		}
		items := append(append([]*obj{}, retainAll(a0.fields)...), retainAll(a1.fields)...)
		return m.ok(m.sized(mkArray(items)))
	case "array.Array.tryAppendAll":
		if m.faultTick() {
			return m.allocError()
		}
		a0.fields = append(a0.fields, retainAll(a1.fields)...)
		m.chargeMemory(a0)
		return m.ok(unitObj)
	case "array.Array.tryInsertAt":
		at, n := uint64(a1.i), uint64(len(a0.fields))
		if m.faultTick() {
			return m.allocError()
		}
		if at > n {
			return m.ok(mkBool(false))
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
	case "array.Array.tryClone", "array.Array.tryReverse":
		return m.tryCopyItems(a0, 0, int64(len(a0.fields)), key == "array.Array.tryReverse")
	case "prelude.AllocError.wrap":
		if m.faultTick() {
			return m.allocError()
		}
		return m.ok(retain(a[0]))
	case "prelude.String.tryConcat":
		if m.faultTick() {
			return m.allocError()
		}
		return m.ok(mkStr(append(append([]byte{}, a0.str()...), a1.str()...)))
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
	case "map.Map.tryClone":
		if m.faultTick() {
			return m.allocError()
		}
		return m.ok(m.copy(a0))
	case "map.Map.tryEntries":
		return m.tryMapEntries(a0)
	case "map.tryEmpty":
		return m.tryEmptyRecord("std/map", "Map", 3)
	case "text.StringBuilder.tryNew":
		return m.tryEmptyRecord("std/text", "StringBuilder", 1)
	}
	return nil
}

func (m *Machine) tryCopyItems(src *obj, start, end int64, reverse bool) *obj {
	if m.faultTick() {
		return m.allocError()
	}
	return m.ok(copyItems(m, src, start, end, reverse))
}

func (m *Machine) tryOptionalCopy(src *obj, start, end int64) *obj {
	if m.faultTick() {
		return m.allocError()
	}
	return m.ok(m.some(copyItems(m, src, start, end, false)))
}

func copyItems(m *Machine, src *obj, start, end int64, reverse bool) *obj {
	out := make([]*obj, 0, end-start)
	for i := start; i < end; i++ {
		at := i
		if reverse {
			at = end - 1 - (i - start)
		}
		out = append(out, m.copy(src.fields[at]))
	}
	c := mkArray(out)
	m.chargeMemory(c)
	return c
}

func (m *Machine) tryMapEntries(mp *obj) *obj {
	if m.faultTick() {
		return m.allocError()
	}
	keys, values := deref(mp.fields[0]), deref(mp.fields[1])
	entry := m.lookup("std/map", "Entry")
	out := make([]*obj, len(keys.fields))
	for i := range out {
		out[i] = mkRecord(entry, []*obj{m.copy(keys.fields[i]), m.copy(values.fields[i])})
	}
	c := mkArray(out)
	m.chargeMemory(c)
	return m.ok(c)
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

func retainAll(items []*obj) []*obj {
	out := make([]*obj, len(items))
	for i, x := range items {
		out[i] = retain(x)
	}
	return out
}

func (m *Machine) sized(v *obj) *obj {
	m.chargeMemory(v)
	return v
}
